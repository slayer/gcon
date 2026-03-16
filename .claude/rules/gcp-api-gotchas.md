# GCP API Gotchas

## Cloud Monitoring: Data points returned newest-first

`ListTimeSeries` returns points in **descending** timestamp order (newest first). UI code that treats `values[len-1]` as "current" will be inverted — sparklines render backwards and stats show the oldest value as current.

Always sort ascending by timestamp before returning from metric-fetching functions:

```go
sort.Slice(dataPoints, func(i, j int) bool {
    return dataPoints[i].Timestamp.Before(dataPoints[j].Timestamp)
})
```

This applies to both `fetchMetricData` (Compute Engine) and `fetchCloudRunMetric` / `fetchPercentileData` (Cloud Run). The sort should happen at the data layer, not in UI consumers.

## Cloud Run: `request_latencies` metric is already in milliseconds

`run.googleapis.com/request_latencies` returns distribution values in **milliseconds**, not seconds. Do not multiply by 1000 — the data is ready to use as-is.

```go
// Wrong — values are already in ms, this inflates 27ms → 27000ms → displayed as "27s"
scaleToMs(p50)  // multiplies by 1000

// Correct — use raw values directly, they're in milliseconds
return p50, p95, p99, nil
```

The `LatencyYLabel` formatter in `metricchart` expects milliseconds: values < 1000 display as "Xms", values >= 1000 display as "X.Ys". Double-scaling produces absurd values like "27214.6s".

## Cloud SQL: `state` field means capability, not running status

GCP's `state` field on `DatabaseInstance` represents operational capability, not whether the instance is currently running. Per the docs: `RUNNABLE` = "running, **or has been stopped by owner**".

The actual running vs stopped distinction comes from `activationPolicy`:
- `ALWAYS` = instance is running
- `NEVER` = instance is stopped by owner
- `ON_DEMAND` = starts on connection (deprecated)

To get the display state that matches the GCP web console, reconcile both fields:

```go
func effectiveState(inst *sqladmin.DatabaseInstance) string {
    if inst.Settings != nil && inst.State == "RUNNABLE" && inst.Settings.ActivationPolicy == "NEVER" {
        return "STOPPED"
    }
    return inst.State
}
```

## ForceSendFields required for Patch operations

Go's JSON serialization omits zero-value fields by default. The Google API Go client uses `ForceSendFields` to explicitly include specific fields in the JSON body. Without it, a Patch request sends empty values for every other field in the struct, which GCP rejects.

Always use `ForceSendFields` when doing partial updates via Patch:

```go
// Wrong — sends zero values for Tier, DataDiskSizeGb, etc.
Settings: &sqladmin.Settings{
    ActivationPolicy: "NEVER",
},

// Correct — only sends activationPolicy in the JSON body
Settings: &sqladmin.Settings{
    ActivationPolicy: "NEVER",
    ForceSendFields:  []string{"ActivationPolicy"},
},
```

This applies to any GCP API Patch operation, not just Cloud SQL.

## IAM: `PrivateKeyData` is base64-encoded

When creating a service account key via `projects.serviceAccounts.keys.create`, the response's `PrivateKeyData` field contains the JSON key file **base64-encoded**, not raw JSON. You must decode it before writing to disk.

```go
// Wrong — writes base64 gibberish to the key file
os.WriteFile("key.json", []byte(key.PrivateKeyData), 0600)

// Correct — decode first, then write valid JSON
keyJSON, err := base64.StdEncoding.DecodeString(key.PrivateKeyData)
if err != nil {
    return fmt.Errorf("decode private key data: %w", err)
}
os.WriteFile("key.json", keyJSON, 0600)
```

The resulting JSON file should contain `type`, `project_id`, `private_key_id`, `private_key`, etc. If you see a base64 string instead, you forgot to decode.

## Cloud Run: Container array replacement on Patch

The Cloud Run v2 API replaces the entire `template.containers` array atomically when the update mask includes `template.containers`. Unlike field-level patching, this means **every field in the container must be populated** in the request — any omitted field resets to its zero value.

This is especially dangerous for:
- **Secret-ref environment variables**: If the edit form excludes secret refs (since users can't edit them), they get deleted on update
- **Non-modeled fields**: `LivenessProbe`, `StartupProbe`, `VolumeMounts`, `WorkingDir` are wiped if not re-sent
- **Ports, resources, command/args**: Omitting these clears them silently

```go
// Wrong — only sets image, all other container fields reset to zero
container := &run.GoogleCloudRunV2Container{
    Image: "gcr.io/project/image:v2",
}
template.Containers = []*run.GoogleCloudRunV2Container{container}

// Correct — clone original, overlay only changed fields
clone := *originalContainer  // shallow copy preserves probes, volumes, etc.
container := &clone
container.Image = "gcr.io/project/image:v2"
template.Containers = []*run.GoogleCloudRunV2Container{container}
```

For env vars, secret-ref env vars (`ValueSource != nil`) must be merged back from the original:

```go
// Re-add secret refs from original alongside plain-text updates
for _, origEnv := range originalContainer.Env {
    if origEnv.ValueSource != nil {
        envs = append(envs, origEnv)
    }
}
```

**Rule**: When updating containers, always start from the existing container spec and modify individual fields. Never build a container from scratch with only the changed fields.

## Cloud Run: VPC Access mutual exclusivity (Connector vs NetworkInterfaces)

The Cloud Run v2 API's `VpcAccess` struct has two mutually exclusive fields: `Connector` (Serverless VPC Access connector) and `NetworkInterfaces` (Direct VPC egress). The API enforces **exactly one** must be set — sending both (even if one is empty with `ForceSendFields`) causes HTTP 400.

This is dangerous during edits because `template.vpcAccess` in the update mask replaces the entire VPC access block atomically (same pattern as containers). Sending a partial `VpcAccess` with only `Egress` wipes the original `NetworkInterfaces` or `Connector`.

```go
// Wrong — always sends VPC access, even if unchanged
// On a service with Direct VPC egress (NetworkInterfaces), this sends
// Connector="" which conflicts with the original NetworkInterfaces
if vpc == "" {
    update.VPCConnector = &empty  // triggers VpcAccess block in patch
}

// Correct — only include VPC fields when user actually changed them
if vpc != origConnector {
    update.VPCConnector = &vpc
}
if egress != origEgressRaw {
    update.VPCEgress = &egress
}
```

**Rule**: For sub-objects that the API replaces atomically (`vpcAccess`, `containers`), only include them in the update when values actually changed. Compare against the original to detect real changes.

## Compute Engine: Instance state checks — use IsStopped() not == "RUNNING"

GCP Compute instances have multiple states beyond RUNNING and STOPPED: `PROVISIONING`, `STAGING`, `SUSPENDED`, `STOPPING`, `SUSPENDING`, `REPAIRING`. Operations like machine type changes require the instance to be in TERMINATED or STOPPED state specifically.

Checking `status == "RUNNING"` misses SUSPENDED and other non-stopped states, allowing the API call to proceed and fail with a cryptic 400 error.

```go
// Wrong — only catches RUNNING, misses SUSPENDED/PROVISIONING/etc
if instance.Status == "RUNNING" {
    return errors.New("must be stopped")
}

// Correct — use the IsStopped() helper, catches all non-stopped states
if !instance.IsStopped() {
    return fmt.Errorf("must be stopped (current status: %s)", instance.Status)
}
```

Both `Instance` and `InstanceDetails` have `IsStopped()` which checks for `TERMINATED || STOPPED`.

## ForceSendFields required for Create operations too

`ForceSendFields` is needed not just for Patch but also for Create when intentionally setting zero values. Common case: `MinInstanceCount = 0` (scale-to-zero) is omitted from JSON because `int64(0)` is a zero value.

```go
// Wrong — MinInstanceCount=0 omitted, API may use a non-zero default
scaling := &run.GoogleCloudRunV2RevisionScaling{
    MinInstanceCount: 0,
}

// Correct — explicitly include zero value
scaling := &run.GoogleCloudRunV2RevisionScaling{
    MinInstanceCount: 0,
    ForceSendFields:  []string{"MinInstanceCount"},
}
```
