# GCP API Gotchas

## GKE: No `kubernetes.io/cluster/*` metric family exists

GKE has no cluster-aggregate metrics under `kubernetes.io/cluster/*`. Querying for
`kubernetes.io/cluster/cpu/allocatable_utilization`, `.../memory/allocatable_utilization`,
`.../node_count`, or `.../pod_count` returns:

> `NotFound: Cannot find metric(s) that match type = "kubernetes.io/cluster/..."`

The actual metric families are scoped per-resource:

- `kubernetes.io/node/*` — k8s_node resource (CPU, memory, network, allocatable cores)
- `kubernetes.io/pod/*` — k8s_pod resource (network, volume)
- `kubernetes.io/container/*` — k8s_container resource
- `kubernetes.io/anthos/*` — Anthos-only

To get cluster-level numbers, reduce node/pod series cross-series:

```go
// Cluster average CPU: per-node 0–1 ratio, REDUCE_MEAN across nodes, ×100
filter := `resource.type = "k8s_node" AND resource.labels.cluster_name = "..." AND
           metric.type = "kubernetes.io/node/cpu/allocatable_utilization"`
// Aligner: ALIGN_MEAN, Reducer: REDUCE_MEAN

// Node count: each running node has one allocatable_cores series; count them
filter := `resource.type = "k8s_node" AND ... AND
           metric.type = "kubernetes.io/node/cpu/allocatable_cores"`
// Aligner: ALIGN_MEAN, Reducer: REDUCE_COUNT

// Pod count: each running pod emits received_bytes_count (cumulative, even at rate 0)
filter := `resource.type = "k8s_pod" AND ... AND
           metric.type = "kubernetes.io/pod/network/received_bytes_count"`
// Aligner: ALIGN_RATE, Reducer: REDUCE_COUNT
```

**Rule**: when adding GKE metrics, verify the metric exists in
[GCP's Kubernetes metric catalog](https://cloud.google.com/monitoring/api/metrics_kubernetes)
before using it. If the design says "cluster-level X", the implementation
will almost always be "node-level X with REDUCE_MEAN/REDUCE_SUM/REDUCE_COUNT".

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

## Compute Engine: Subnet `State` field can be empty

GCP's `Subnetwork.State` field is not always populated. Active subnets in `READY` state may return an empty string instead of `"READY"`. Always provide a fallback when displaying:

```go
// Wrong — shows blank "Status:" for active subnets
b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Status", d.Status))

// Correct — default to READY when empty
b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Status", defaultIfEmpty(d.Status, "READY")))
```

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

## Cloud Monitoring: No `OR` between `resource.type` values in filters

GCP's filter language allows `OR` between `metric.labels.*` or `metadata.*` restrictions but **not** between resource types. Building a filter like:

```
(resource.type = "https_lb_rule" OR resource.type = "internal_http_lb_rule") AND ...
```

fails with: *"Within the 'resource' prefix, OR can only be used to connect a list of 'labels' restrictions or a list of 'metadata' restrictions."* The query returns empty data with no obvious error in the UI — every chart looks "empty but loaded" until you log the API response.

**Rule**: pick exactly one `resource.type` per fetch. If a feature can be backed by multiple resource types (e.g. external vs. internal HTTPS LBs use `https_lb_rule` vs `internal_http_lb_rule`), pick the right one at the call site and thread it through:

```go
// One filter per resource type — dispatch from caller, not inside the helper
func resourceTypeForLB(r *gcp.ForwardingRule) string {
    switch r.Type {
    case "HTTPS (external)", "HTTP (external)":
        return "https_lb_rule"
    case "HTTPS (internal)", "HTTP (internal)":
        return "internal_http_lb_rule"
    }
    return ""
}

filter := fmt.Sprintf(
    `resource.type = "%s" AND resource.labels.forwarding_rule_name = "%s" AND metric.type = "%s"`,
    resourceType, ruleName, metricType,
)
```

## Cloud Monitoring: `response_code_class` is INTEGER on LB metrics, STRING on Cloud Run

The same label name carries different types across metric families. On `loadbalancing.googleapis.com/https/*` metrics, `response_code_class` is **integer** (`4`, `5`); on `run.googleapis.com/request_count`, it's **string** (`"4xx"`, `"5xx"`).

Sending the wrong syntax gets rejected with: *"The label 'response_code_class' is typed as an integer, but the supplied value '4xx' cannot be parsed as an integer."*

```go
// Wrong on LB metrics — quoted value fails the integer-type check
filter := `... AND metric.labels.response_code_class = "4xx"`

// Correct on LB metrics — unquoted integer
filter := `... AND metric.labels.response_code_class = 4`

// Correct on Cloud Run metrics — quoted string
filter := `... AND metric.labels.response_code_class = "4xx"`
```

**Rule**: keep separate filter helpers for string-typed vs integer-typed labels. Don't reuse Cloud Run patterns when adding LB metrics (or vice-versa) — the label name is a false friend.

## Compute Engine: Regional NEGs need `RegionNetworkEndpointGroups`, not `NetworkEndpointGroups`

The Compute SDK exposes three sibling services for network endpoint groups:

- `GlobalNetworkEndpointGroups` — global scope
- `NetworkEndpointGroups` — **zonal** (`projects/X/zones/{zone}/networkEndpointGroups/...`)
- `RegionNetworkEndpointGroups` — **regional** (`projects/X/regions/{region}/networkEndpointGroups/...`)

Treating "non-global" as a single bucket and always calling `NetworkEndpointGroups.Get(projectID, location, name)` works for zonal NEGs but fails for regional ones with a 404 — masking any downstream logic that depends on the NEG metadata (e.g. `NetworkEndpointType == "SERVERLESS"` detection).

This matters: **serverless backends (Cloud Run / Cloud Functions / App Engine) are typically regional NEGs.**

```go
// Parse the kind from the URL path (/zones/, /regions/, /global/), not from
// the location string format — zone vs region can't be reliably distinguished
// by suffix.
func groupScope(url string) (location, kind string) {
    parts := strings.Split(url, "/")
    for i, p := range parts {
        switch p {
        case "zones":   if i+1 < len(parts) { return parts[i+1], "zone" }
        case "regions": if i+1 < len(parts) { return parts[i+1], "region" }
        case "global":  return "global", "global"
        }
    }
    return "", ""
}

// Dispatch to the right service per kind
switch locationKind {
case "global": c.service.GlobalNetworkEndpointGroups.Get(projectID, name)
case "region": c.service.RegionNetworkEndpointGroups.Get(projectID, location, name)
case "zone":   c.service.NetworkEndpointGroups.Get(projectID, location, name)
}
```

**Rule**: when adding any NEG-related call, always check whether the resource can be regional. A `zones/regions` split in the URL is the authoritative source.

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

## Prefer one zone/region list over N per-resource `Get` calls

GCP "list things in a group" patterns (managed instance groups, target
pools, instance group memberships, etc.) often return only resource
URLs / names + status. Building a full projection by calling
`Resource.Get(name)` per item is the natural shape — and it's the
common N+1 trap. On a 500-node MIG that's 500 sequential round trips
(~30s of unnecessary latency, and an easy way to hit per-second
Compute quotas) just to render one tab.

The cheaper shape: list every resource in the surrounding scope (zone
or region) **once**, then join by name in a map.

```go
// Wrong — N+1: one Get per managed instance, sequentially inside the
// page callback. 500 nodes = 500 round trips before the user sees
// anything.
err := req.Pages(ctx, func(page *compute.InstanceGroupManagersListManagedInstancesResponse) error {
    for _, mi := range page.ManagedInstances {
        inst, err := c.service.Instances.Get(projectID, zone, shortName(mi.Instance)).Do()
        // ... project inst ...
    }
    return nil
})

// Correct — list every instance in the zone once, hash by name.
// Two paginated calls total regardless of MIG size; trades response
// payload for round trips, which is the right call since Compute
// quotas are per-call, not per-byte.
byName := map[string]*compute.Instance{}
err := c.service.Instances.List(projectID, zone).Pages(ctx, func(page *compute.InstanceList) error {
    for _, inst := range page.Items {
        byName[inst.Name] = inst
    }
    return nil
})
// ... then iterate the managed-instance list and look up byName ...
```

**Rule**: any time you find yourself writing a per-item `Get` inside a
list-iteration loop, check whether a parent-scope `List(zone)` or
`List(region)` returns the same data in one shot. The zone/region call
is almost always cheaper, even if it returns more resources than you
strictly need — map lookup is O(1) and the wasted bytes are negligible
vs. the round-trip count.

This also applies to:
- IAM `Members.Get` inside a project's policy-binding iteration → use
  one `Projects.GetIamPolicy` and walk bindings.
- Network endpoint groups → use one `NetworkEndpointGroups.List` per
  zone/region instead of per-NEG `Get`.
- Cloud SQL `Databases.Get` inside an instance iteration → use
  `Databases.List(instance)` once.

The exception is when the parent-scope list itself doesn't include the
metadata you need (some APIs strip fields from list responses). In
that case, falling back to per-item `Get` is unavoidable — but
parallelize it via a worker pool, don't run it serially.
