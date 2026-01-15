# Disk Snapshots Management - Documentation

## Summary

Implemented comprehensive disk snapshot management features in gcon, allowing users to list, view, and delete GCP disk snapshots through the TUI.

## Changes Made

### 1. GCP API Client Layer (`internal/gcp/compute.go`)

#### New Types
- **`Snapshot`**: Simplified snapshot representation
  - Name, SourceDisk, SizeGB, Status, CreatedAt, StorageBytes
- **`SnapshotDetails`**: Comprehensive snapshot information
  - Basic info (Name, ID, Description, Status, CreatedAt, Labels)
  - Source (SourceDisk, SourceDiskID, SourceDiskZone)
  - Size (SizeGB, StorageBytes, StorageBytesGb, StorageLocations)
  - Encryption (SnapshotEncryptionKey, SourceDiskEncryption)
  - Additional (AutoCreated, ChainName, SatisfiesPZS, SnapshotType)

#### New Functions
- `ListSnapshots(ctx, projectID)`: Returns all snapshots in a project
- `GetSnapshotDetails(ctx, projectID, snapshotName)`: Returns comprehensive snapshot details
- `DeleteSnapshot(ctx, projectID, snapshotName)`: Deletes a snapshot
- Helper methods: `IsReady()`, `IsCreating()`, `IsFailed()` on Snapshot type

#### Tests
- `TestSnapshotFromAPI`: Tests snapshot conversion from API
- `TestSnapshotMethods`: Tests snapshot helper methods
- `TestSnapshotDetailsFromAPI`: Tests detailed snapshot conversion
- All tests pass successfully

### 2. Snapshots List View (`internal/ui/views/snapshots.go`)

#### Features
- Table-based list display with columns:
  - Name (with status indicator icon)
  - Source Disk
  - Size (GB)
  - Created (formatted timestamp)
  - Status
- Status icons: 🟢 (READY), 🟡 (CREATING/UPLOADING), 🔴 (FAILED)
- Keyboard shortcuts:
  - `enter`: View snapshot details
  - `D`: Delete snapshot (with confirmation)
  - `/`: Filter/search
  - `r`: Refresh list
  - `esc`: Back to previous view

#### Implementation Details
- Follows established pattern from `DisksView`
- Uses Bubble Tea table component
- Async API calls with tea.Cmd
- Loading and error states handled
- Empty state messaging

#### Tests (`internal/ui/views/snapshots_test.go`)
- `TestSnapshotStatusIcon`: Status icon rendering
- `TestSnapshotToRow`: Row conversion logic
- `TestNewSnapshotsView`: View initialization
- `TestSnapshotsViewLoadedState`: Data loading
- `TestSnapshotsViewErrorState`: Error handling
- `TestFindSnapshotByName`: Snapshot lookup
- `TestSnapshotsViewRendering`: Various rendering states

### 3. Snapshot Details View (`internal/ui/views/snapshot_details.go`)

#### Features
- Scrollable viewport with comprehensive information display
- Sections:
  - Basic Information (Name, ID, Description, Status, Created)
  - Labels (key-value pairs)
  - Source Information (Disk name and zone)
  - Size and Storage (Disk size, storage bytes, locations)
  - Encryption (Snapshot and source disk encryption types)
  - Additional Info (Auto-created, chain name, PZS)
- Status indicators with colored icons
- Key bindings:
  - `↑/k`, `↓/j`: Scroll up/down
  - `r`: Refresh details
  - `D`: Delete snapshot
  - `esc`: Back to snapshots list

#### Implementation Details
- Uses viewport for scrollable content
- Context-aware sizing
- Timezone-aware timestamp formatting
- Encryption type detection (Google-managed, CMEK, CSEK)
- Storage size conversion (bytes to GB)

#### Tests (`internal/ui/views/snapshot_details_test.go`)
- View initialization
- Loading state rendering
- Status icon rendering for various states
- Snapshot name getter

### 4. Navigation and Sidebar Integration

#### App Integration (`internal/ui/app.go`)
- Added `ViewSnapshots` and `ViewSnapshotDetails` to ViewType enum
- Added snapshot views to App struct
- Added `selectedSnapshot` to selected context
- Navigation message handler for `SnapshotSelectedMsg`
- Recent tracking integration for snapshots
- Sidebar active view highlighting for snapshot views
- View rendering in `renderCurrentView()`
- View size updates in `updateViewSizes()`
- Message delegation to snapshot views in Update()
- Header category display for snapshot views

#### Sidebar Menu (`internal/ui/components/sidebar/menu.go`)
- Added `ViewSnapshots` to ViewType enum
- Added "Snapshots" menu item under Compute Engine
- Uses disk icon (matching disks menu item)
- Proper drill-down navigation

#### Sidebar Navigation (`internal/ui/app.go`)
- Added `ViewSnapshots` case in `handleSidebarNavigation()`
- Initializes `snapshotsView` on first access
- Maintains view state across navigation

#### Command Palette (`internal/ui/components/commandpalette/`)
- Added `ViewSnapshots` to command palette ViewType
- Added "Compute Engine: Snapshots" navigation command
- Added `RecentTypeSnapshot` for recent item tracking
- Recent snapshots appear in command palette

#### Test Updates
- Updated sidebar tests to expect 3 children under Compute Engine (was 2)
- All tests now pass

### 5. Code Quality

- All tests pass (100+ tests)
- Linter passes with 0 issues
- Code formatted with `go fmt`
- Follows project conventions from CLAUDE.md
- Consistent with established patterns
- No breaking changes to existing functionality

## Architecture Decisions

### 1. Pattern Consistency
Followed the exact patterns established by the existing disk management features:
- GCP client layer structure
- Table-based list view
- Scrollable details view
- Navigation message flow
- Context propagation

### 2. User Experience
- Consistent keyboard shortcuts across views
- Status indicators using existing symbol system
- Loading states with spinners
- Error handling with retry capability
- Filter/search functionality
- Responsive to terminal size changes

### 3. Data Flow
```
User Action → View Update → tea.Msg → App Update → Navigation/State Change
                                    ↓
                              GCP API Call (async)
                                    ↓
                              Result Message → View Update
```

### 4. Safety Measures
- Delete operations require user confirmation (handled by parent app)
- Async operations don't block UI
- Error messages are user-friendly
- Status is always visible

## Testing

### Unit Tests Coverage
- GCP client functions (snapshots API)
- Snapshot conversion functions
- View initialization
- State transitions
- Rendering logic
- Helper functions

### Integration Points Tested
- Message flow between views
- Navigation transitions
- Command palette integration
- Sidebar menu structure
- Recent item tracking

## Usage

### Accessing Snapshots View

1. **Via Sidebar**:
   - Navigate to "Compute Engine" → "Snapshots"
   - Or press `1` then `3` (number shortcuts)

2. **Via Command Palette**:
   - Press `:` or `Ctrl+K`
   - Type "snapshots"
   - Select "Compute Engine: Snapshots"

3. **Via Recent Items** (after viewing a snapshot):
   - Press `:` or `Ctrl+K`
   - Recent snapshots appear at top

### Key Bindings

**Snapshots List:**
- `enter` - View snapshot details
- `D` - Delete snapshot
- `/` - Filter/search
- `r` - Refresh
- `esc` - Back

**Snapshot Details:**
- `↑/k`, `↓/j` - Scroll
- `r` - Refresh
- `D` - Delete
- `esc` - Back

## Future Enhancements

Potential improvements for future iterations:

1. **Create Snapshot**: Add ability to create new snapshots from disks
2. **Restore from Snapshot**: Create new disk from snapshot
3. **Snapshot Scheduling**: View/manage snapshot schedules
4. **Multi-region Snapshots**: Better visualization of snapshot locations
5. **Snapshot Metrics**: Show usage statistics and costs
6. **Bulk Operations**: Select multiple snapshots for batch delete

## Files Modified

### New Files
- `internal/ui/views/snapshots.go` (303 lines)
- `internal/ui/views/snapshots_test.go` (179 lines)
- `internal/ui/views/snapshot_details.go` (348 lines)
- `internal/ui/views/snapshot_details_test.go` (70 lines)
- `doc/2026-01-15-disk-snapshots/TODO.md`
- `doc/2026-01-15-disk-snapshots/Documentation.md`

### Modified Files
- `internal/gcp/compute.go` (+130 lines)
- `internal/gcp/compute_test.go` (+168 lines)
- `internal/ui/app.go` (+32 lines, formatting changes)
- `internal/ui/components/sidebar/menu.go` (+2 lines)
- `internal/ui/components/sidebar/sidebar_test.go` (3 test assertions)
- `internal/ui/components/commandpalette/commands.go` (+7 lines)
- `internal/ui/components/commandpalette/recent.go` (+2 lines)

### Total Impact
- **New code**: ~900 lines
- **Modified code**: ~200 lines
- **Tests added**: 24 new test cases
- **No breaking changes**

## Technical Notes

### Snapshot Lifecycle States
- `READY`: Snapshot is complete and can be used
- `CREATING`: Snapshot creation in progress
- `UPLOADING`: Snapshot data being uploaded
- `FAILED`: Snapshot creation failed
- `DELETING`: Snapshot deletion in progress

### Encryption Handling
The implementation detects three encryption types:
1. **Google-managed**: Default GCP encryption (most common)
2. **Customer-managed (CMEK)**: Uses Cloud KMS keys
3. **Customer-supplied (CSEK)**: User-provided encryption keys

### Storage Calculation
- Snapshots are incremental (only changed blocks stored)
- `StorageBytes` may be less than `SizeGB` due to incremental nature
- UI displays both values for transparency

## Performance Considerations

- Snapshots API calls are async (non-blocking)
- List view uses pagination internally (handled by GCP API)
- Table rendering is efficient (Bubble Tea optimization)
- No unnecessary re-renders or API calls
- View state is cached (only refreshes on explicit user action)

## Conclusion

Successfully implemented a complete disk snapshot management feature that:
- Integrates seamlessly with existing codebase
- Maintains consistent UX patterns
- Provides comprehensive functionality (list, view, delete)
- Includes thorough test coverage
- Follows all project conventions
- Passes all quality checks

The feature is production-ready and provides a solid foundation for future snapshot-related enhancements.
