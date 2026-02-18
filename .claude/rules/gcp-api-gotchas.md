# GCP API Gotchas

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
