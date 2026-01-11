# VM Instance Details View

## Task Description

Implement a detailed view for VM instances that displays comprehensive information similar to Google Cloud Console. When a user presses Enter on a VM instance in the instances list, they should navigate to a detailed view showing all instance properties.

## User Story

As a user, I want to press Enter on a VM instance to see a detailed view of that instance with all its properties (similar to Google Cloud Console), so that I can inspect VM configuration without leaving the terminal.

## Implementation Plan

### Phase 1: Extend GCP Client (Data Layer)

**1.1 Create InstanceDetails struct** (`internal/gcp/compute.go`)

Extend the GCP layer to capture more instance data. Create a new `InstanceDetails` struct that includes:

- **Basic Information**
  - Name, InstanceID, Description, Status, CreatedAt, Zone
  - BootDiskSourceImage, DeletionProtection
  - Labels (map), Tags (array)

- **Machine Configuration**
  - MachineType (full name with vCPUs/Memory)
  - CpuPlatform, MinCpuPlatform
  - DisplayDevice enabled
  - GPUs (GuestAccelerators)

- **Networking**
  - CanIpForward
  - NetworkInterfaces array with:
    - Name, Network, Subnetwork, NicType
    - InternalIP, ExternalIP, StackType
    - NetworkTier, IPForwarding

- **Storage**
  - Disks array with:
    - Name, SizeGB, Type (from source)
    - Boot flag, Mode (READ_WRITE/READ_ONLY)
    - AutoDelete flag

- **Security**
  - ShieldedInstanceConfig (SecureBoot, vTPM, IntegrityMonitoring)
  - ServiceAccount, Scopes

- **Availability/Scheduling**
  - ProvisioningModel (Standard/Spot)
  - Preemptible flag
  - OnHostMaintenance (MIGRATE/TERMINATE)
  - AutomaticRestart flag

**1.2 Create GetInstanceDetails method**

Add a new method to `ComputeClient` that returns full `InstanceDetails`:

```go
func (c *ComputeClient) GetInstanceDetails(ctx context.Context, projectID, zone, instanceName string) (*InstanceDetails, error)
```

This method will call `Instances.Get()` and transform the full API response into our struct.

### Phase 2: Create Instance Details View (UI Layer)

**2.1 Create new view file** (`internal/ui/views/instance_details.go`)

Create a scrollable details view with sections:

```
┌─────────────────────────────────────────────────────────────┐
│ Instance: stage-db                                    🟢 RUNNING │
├─────────────────────────────────────────────────────────────┤
│ Basic Information                                           │
│ ─────────────────                                           │
│ Name:              stage-db                                 │
│ Instance ID:       4006516603965718780                      │
│ Status:            Running                                  │
│ Zone:              us-central1-b                            │
│ Created:           Sep 4, 2025, 2:55:00 PM UTC+03:00        │
│ Description:       None                                     │
│ Deletion protection: Disabled                               │
│                                                             │
│ Labels:                                                     │
│   goog-terraform: true                                      │
│                                                             │
│ Tags:                                                       │
│   allow-lan-mongo, allow-lan-mysql, ...                     │
│                                                             │
│ Machine Configuration                                       │
│ ────────────────────                                        │
│ Machine Type:      n4-custom-4-20480 (4 vCPUs, 20 GB)       │
│ CPU Platform:      Intel Emerald Rapids                     │
│ Min CPU Platform:  None                                     │
│ Display Device:    Disabled                                 │
│ GPUs:              None                                     │
│                                                             │
│ Networking                                                  │
│ ──────────                                                  │
│ ┌─────┬────────┬───────────┬────────┬──────────────┐        │
│ │ NIC │ Network│ Subnetwork│ Type   │ Internal IP  │        │
│ ├─────┼────────┼───────────┼────────┼──────────────┤        │
│ │nic0 │ main   │ c1-main   │ GVNIC  │ 10.200.0.30  │        │
│ └─────┴────────┴───────────┴────────┴──────────────┘        │
│ IP Forwarding:     Off                                      │
│                                                             │
│ Storage                                                     │
│ ───────                                                     │
│ ┌──────────────┬────────┬──────────────────┬───────────┐    │
│ │ Name         │ Size   │ Type             │ Mode      │    │
│ ├──────────────┼────────┼──────────────────┼───────────┤    │
│ │stage-db-root │ 750 GB │ Hyperdisk Balanced│ Read/write│    │
│ │ (Boot)       │        │                  │           │    │
│ └──────────────┴────────┴──────────────────┴───────────┘    │
│                                                             │
│ Security & Access                                           │
│ ─────────────────                                           │
│ Secure Boot:       Off                                      │
│ vTPM:              On                                       │
│ Integrity Monitor: On                                       │
│                                                             │
│ Service Account:   719866551912-compute@...                 │
│ Scopes:            Custom access                            │
│                                                             │
│ Availability Policies                                       │
│ ─────────────────────                                       │
│ Provisioning:      Standard                                 │
│ Preemptibility:    Off                                      │
│ On Host Maint:     Migrate VM                               │
│ Auto Restart:      On                                       │
└─────────────────────────────────────────────────────────────┘
│ ↑/↓ scroll • s start • x stop • R reset • esc back          │
```

**View structure:**
- Use `viewport.Model` from bubbles for scrolling
- Organize content in collapsible/expandable sections
- Support keyboard navigation (j/k or arrows for scrolling)
- Keep action keys (s/x/R) available from details view

**2.2 Define message types**

```go
// InstanceSelectedMsg triggers navigation to instance details
type InstanceSelectedMsg struct {
    Instance gcp.Instance // Basic info, enough to fetch details
}

// InstanceDetailsLoadedMsg contains full instance details
type instanceDetailsLoadedMsg struct {
    details *gcp.InstanceDetails
}

// InstanceDetailsErrorMsg indicates a fetch error
type instanceDetailsErrorMsg struct {
    err error
}
```

### Phase 3: Wire Up Navigation (App Layer)

**3.1 Update ViewType enum** (`internal/ui/app.go`)

```go
const (
    ViewProjects ViewType = iota
    ViewInstances
    ViewInstanceDetails  // NEW
    ViewBuckets
    ViewLogs
)
```

**3.2 Add instanceDetailsView to App struct**

```go
type App struct {
    // ... existing fields ...
    instanceDetailsView *views.InstanceDetailsView
}
```

**3.3 Handle InstanceSelectedMsg in App.Update()**

When receiving `InstanceSelectedMsg`:
1. Store selected instance reference
2. Create new `InstanceDetailsView` with project ID, zone, instance name
3. Set view size
4. Switch to `ViewInstanceDetails`
5. Return `instanceDetailsView.Init()`

**3.4 Update back navigation**

Modify the back handler:
- From `ViewInstanceDetails` → return to `ViewInstances`
- From `ViewInstances` → return to `ViewProjects` (existing)

**3.5 Update instances view to emit selection message**

In `InstancesView.Update()`, handle Enter key to emit `InstanceSelectedMsg`:

```go
case tea.KeyMsg:
    if key.Matches(msg, key.NewBinding(key.WithKeys("enter"))) {
        if item, ok := v.list.SelectedItem().(instanceItem); ok {
            return views.InstanceSelectedMsg{Instance: item.instance}
        }
    }
```

### Phase 4: Implement Rendering

**4.1 Section rendering helpers**

Create helper functions for consistent section rendering:
- `renderSection(title string, rows []Row) string`
- `renderTable(headers []string, rows [][]string) string`
- `renderKeyValue(key, value string) string`

**4.2 Format helpers**

- `formatTimestamp(iso string) string` - Pretty print timestamps
- `formatBytes(bytes int64) string` - Format disk sizes
- `formatBool(b bool, trueVal, falseVal string) string` - On/Off, Enabled/Disabled
- `truncateString(s string, max int) string` - Truncate long values

### Phase 5: Testing

**5.1 Unit tests for InstanceDetails transformation**

Test `instanceDetailsFromAPI()` with mock compute.Instance data:
- All fields populated
- Missing/nil optional fields
- Edge cases (empty arrays, nil pointers)

**5.2 Unit tests for view rendering**

- Section rendering
- Scrolling behavior
- Key handling

## File Changes Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/gcp/compute.go` | Modify | Add `InstanceDetails` struct and `GetInstanceDetails()` method |
| `internal/ui/views/instance_details.go` | Create | New detailed instance view with scrollable content |
| `internal/ui/views/instances.go` | Modify | Add Enter key handler to emit `InstanceSelectedMsg` |
| `internal/ui/messages.go` | Modify | Add `InstanceSelectedMsg` type |
| `internal/ui/app.go` | Modify | Add `ViewInstanceDetails`, handle navigation |
| `internal/gcp/compute_test.go` | Create/Modify | Tests for `GetInstanceDetails()` |
| `internal/ui/views/instance_details_test.go` | Create | Tests for details view |

## Questions/Decisions

1. **Scrolling vs Sections**: Should we use a single scrollable viewport or collapsible sections?
   - **Decision**: Start with single scrollable viewport for simplicity

2. **Instance Actions**: Should start/stop/reset actions be available from details view?
   - **Decision**: Yes, keep same key bindings for consistency

3. **Refresh**: Should we auto-refresh details or only on manual refresh?
   - **Decision**: Manual refresh only (same pattern as instances list)

4. **Field Display**: Which fields to show/hide?
   - **Decision**: Show most commonly useful fields, similar to GCP Console

## Progress Tracking

- [ ] Phase 1.1: Create InstanceDetails struct
- [ ] Phase 1.2: Implement GetInstanceDetails method
- [ ] Phase 2.1: Create instance_details.go view
- [ ] Phase 2.2: Define message types
- [ ] Phase 3.1-3.5: Wire up navigation
- [ ] Phase 4: Implement rendering helpers
- [ ] Phase 5: Add tests
- [ ] Final: Manual testing and polish
