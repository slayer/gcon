# List Disks Feature Documentation

## Summary of Changes

Added a new view to list Compute Engine persistent disks in the gcon terminal UI. Users can now navigate to the Disks view from the sidebar menu under "Compute Engine" and see all disks across all zones in the selected project.

## Technical Details

### Files Modified

1. **internal/gcp/compute.go**
   - Added `Disk` struct with fields: Name, Zone, SizeGB, Type, Status, AttachedTo, CreatedAt
   - Added `ListDisks()` method using GCP Compute API's aggregated list endpoint
   - Added `diskFromAPI()` helper to convert API response to internal struct
   - Added `IsAttached()` and `IsReady()` helper methods on Disk

2. **internal/ui/views/disks.go** (new file)
   - Created `DisksView` struct following the pattern from instances.go
   - Table columns: Name, Zone, Size, Type, Attached To
   - Status indicators: green dot (attached), red dot (available), yellow dot (transitioning)
   - Supports filtering via `/` key and refresh via `r` key

3. **internal/ui/app.go**
   - Added `disksView` field to App struct
   - Updated `handleSidebarNavigation()` to create DisksView when navigating to disks
   - Added DisksView to Update() message routing
   - Added DisksView to `renderCurrentView()`
   - Added DisksView sizing in `updateViewSizes()`
   - Added cleanup of disksView in back navigation

### Files Added

- `internal/ui/views/disks.go` - DisksView implementation
- `internal/ui/views/disks_test.go` - Unit tests for DisksView
- `internal/gcp/compute_test.go` - Added tests for disk-related functions

## Usage

1. Launch gcon and select a project
2. Use Tab to focus the sidebar
3. Navigate to "Compute Engine" > "Disks"
4. Use `/` to filter disks by name, zone, or type
5. Use `r` to refresh the disk list
6. Use `esc` to go back

## Key Bindings

| Key | Action |
|-----|--------|
| `/` | Filter disks |
| `r` | Refresh list |
| `j/k` or `↓/↑` | Navigate list |
| `esc` | Go back |

## Status Indicators

- Green dot (●): Disk is attached to an instance
- Red dot (●): Disk is available (not attached)
- Yellow dot (●): Disk is in a transitioning state (creating, etc.)
