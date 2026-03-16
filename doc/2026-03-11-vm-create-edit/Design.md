# VM Instance Create/Edit — Design Document

## Overview

Add form-based VM instance creation and editing to gcon. Tier 1 covers essential fields: name, zone, machine type, boot disk, and networking. The form structure is designed for easy expansion to tier 2 fields (labels, tags, scheduling, etc.).

## Form Structure

### Create VM Instance

```
┌─ Create VM Instance ────────────────────────┐
│                                              │
│ ▼ Basic Settings                             │
│   Name         [my-instance         ]        │
│   Zone         [us-central1-a       ▾]       │
│                                              │
│ ▼ Machine Configuration                      │
│   Machine Type [e2-medium           ▾]       │
│   Custom       [                    ]        │
│                                              │
│ ▼ Boot Disk                                  │
│   Image        [Debian 12           ▾]       │
│   Disk Size GB [10                  ]        │
│   Disk Type    [pd-balanced         ▾]       │
│                                              │
│ ▼ Networking                                 │
│   Network      [default             ▾]       │
│   Subnetwork   [default             ▾]       │
│   External IP  [Ephemeral           ▾]       │
│                                              │
│         [Cancel]  [Ctrl+S: Create]           │
└──────────────────────────────────────────────┘
```

### Edit VM Instance

Same sections but with read-only fields for immutable properties:
- **Read-only**: Name, Zone, Image, Disk Type, Network, Subnetwork
- **Editable**: Machine Type, Disk Size (expand only)
- External IP editing deferred to tier 2

Edit includes a diff preview before applying changes.

### Tier 2 Sections (future, collapsed by default)

```
▶ Labels & Tags         — key/value labels, network tags
▶ Security              — service account, deletion protection
▶ Scheduling            — preemptible/spot, auto-restart
▶ Advanced              — startup script, metadata key/value pairs
```

## Architecture

### Shared Form Logic

`instance_form.go` contains shared form-building and population logic:
- `buildInstanceForm(mode FormMode)` — creates form with all sections
- `populateFromDetails(form, details)` — fills form from existing InstanceDetails
- Curated boot disk images list (Debian 12, Ubuntu 24.04, etc.)

Both create and edit views use these, with edit marking immutable fields as read-only.

### Machine Type Loading

- Zone selection triggers async fetch of `machineTypes.list` for that zone
- Results cached in `map[string][]MachineType` at the view level
- Cache hit → immediate dropdown population, no spinner
- Cache miss → spinner on machine type dropdown while fetching
- Grouped by family (e2, n2, n2d, c3, etc.) in dropdown labels
- Custom text field overrides dropdown selection

### Network/Subnetwork Loading

- Networks fetched from existing VPC client (`ListNetworks`)
- Subnetworks fetched per region (derived from zone: strip `-a`/`-b`/etc.)
- Zone change triggers subnetwork refresh

## GCP Client Layer

### New Types

```go
type InstanceCreateConfig struct {
    Name         string
    Zone         string
    MachineType  string   // full or short name
    ImageProject string   // e.g., "debian-cloud"
    ImageFamily  string   // e.g., "debian-12"
    DiskSizeGB   int64
    DiskType     string   // "pd-balanced", "pd-standard", "pd-ssd"
    Network      string
    Subnetwork   string
    ExternalIP   bool     // true = ephemeral, false = none
}

type MachineType struct {
    Name        string  // "e2-medium"
    Description string  // "2 vCPU, 4 GB"
    CPUs        int64
    MemoryMB    int64
}

type SubnetworkInfo struct {
    Name      string
    Region    string
    IPRange   string
    Network   string
}
```

### New Methods

```go
CreateInstance(ctx, projectID string, config InstanceCreateConfig) error
SetMachineType(ctx, projectID, zone, instance, machineType string) error
ResizeBootDisk(ctx, projectID, zone, diskName string, sizeGB int64) error
ListMachineTypes(ctx, projectID, zone string) ([]MachineType, error)
ListSubnetworks(ctx, projectID, region string) ([]SubnetworkInfo, error)
```

### Edit Strategy

Editing uses separate API calls since GCP has no single "update instance" endpoint:
1. `instances.setMachineType` — requires stopped instance
2. `disks.resize` — online, expand only

Calls run sequentially. If one fails, report which step failed and what succeeded.

## View Layer

### Create View (`instance_create.go`)

- Embeds `CreateViewBase`
- States: Form → Saving (handled by base)
- On zone change: fetch machine types (async with spinner)
- On submit: validate → emit `CreateInstanceMsg`
- App handler calls `CreateInstance()`, returns `InstanceCreateResultMsg`

### Edit View (`instance_edit.go`)

- Manual state machine: Loading → Form → Diff → Saving
- Loading: fetch current instance details + machine types for zone
- Form: pre-populated with current values, immutable fields read-only
- Diff: shows changed fields (machine type, disk size)
- Saving: sequential API calls with progress
- `SetError()` for error propagation from app handlers

### Message Flow

```
Create:
  InstancesView ──c──> InstanceCreateRequestMsg
    → App creates view, navigates
    → form submit → CreateInstanceMsg{config}
    → App calls computeClient.CreateInstance()
    → InstanceCreateResultMsg
    → navigate back, refresh list

Edit:
  InstanceDetailsView ──e──> InstanceEditRequestMsg{zone, name}
    → App creates view, navigates
    → loads current config
    → Ctrl+S → diff preview
    → Enter → InstanceEditSubmitMsg{changes}
    → App calls SetMachineType / ResizeBootDisk
    → InstanceEditResultMsg{partialFailures}
    → navigate back, refresh details
```

## Entry Points

| Trigger | View | Key |
|---------|------|-----|
| Instances list | Create | `c` |
| Instance details | Edit | `e` |
| Command palette | Create | "Compute: Create Instance" |
| Instance details action menu | Edit | `.` → Edit |

## Files

### New Files
- `internal/ui/views/instance_create.go`
- `internal/ui/views/instance_edit.go`
- `internal/ui/views/instance_form.go` (shared form builder)

### Modified Files
- `internal/gcp/compute.go` — new methods and types
- `internal/ui/app.go` — ViewType, fields, getCurrentViewModel, updateViewSizes
- `internal/ui/app_render.go` — renderCurrentView
- `internal/ui/app_navigation.go` — handlers, clearAllViews, sidebar guards
- `internal/ui/views/instances.go` — `c` key
- `internal/ui/views/instance_details.go` — `e` key
- `internal/ui/components/commandpalette/commands.go` — nav command
