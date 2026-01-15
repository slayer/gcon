# Task: Disk Snapshots Management

## Task Description

Implement disk snapshots management features in gcon:
- List all snapshots across all zones
- View detailed snapshot information
- Delete snapshots with confirmation

## Implementation Plan

### 1. GCP API Client Layer (`internal/gcp/compute.go`)

- [x] Add `Snapshot` struct with essential fields
- [x] Add `SnapshotDetails` struct with comprehensive information
- [x] Implement `ListSnapshots(ctx, projectID)` - list all snapshots
- [x] Implement `GetSnapshotDetails(ctx, projectID, snapshotName)` - get snapshot details
- [x] Implement `DeleteSnapshot(ctx, projectID, snapshotName)` - delete a snapshot
- [x] Add tests for snapshot client functions

### 2. Snapshots List View (`internal/ui/views/snapshots.go`)

- [x] Create `SnapshotsView` struct following the pattern from `DisksView`
- [x] Implement table display with columns:
  - Name (with status indicator)
  - Source Disk
  - Size (GB)
  - Created At
  - Status
- [x] Add key bindings:
  - `enter` - view snapshot details
  - `r` - refresh list
  - `/` - filter/search
  - `D` - delete snapshot (with confirmation)
- [x] Handle loading/error states
- [x] Add tests for snapshots view

### 3. Snapshot Details View (`internal/ui/views/snapshot_details.go`)

- [x] Create `SnapshotDetailsView` struct following the pattern from `DiskDetailsView`
- [x] Display comprehensive snapshot information:
  - Basic: Name, ID, Description, Status
  - Source: Source disk name and zone
  - Size: Total size, storage bytes
  - Timestamps: Created, Last used
  - Storage location
  - Labels
  - Encryption info
- [x] Add key bindings:
  - `D` - delete snapshot (with confirmation)
  - `esc` - back to snapshots list
- [x] Add tests for snapshot details view

### 4. Integration & Navigation

- [x] Add `ViewSnapshots` and `ViewSnapshotDetails` to `ViewType` enum in `app.go`
- [x] Add snapshot views to `App` struct
- [x] Handle `SnapshotSelectedMsg` navigation message
- [x] Add "Snapshots" item to sidebar menu
- [x] Update command palette to include snapshot navigation
- [x] Handle back navigation (esc key)

### 5. Testing & Quality

- [x] Run tests for all new code
- [x] Run full test suite to catch regressions
- [x] Run linter (`make lint`)
- [x] Manual testing:
  - List snapshots
  - View snapshot details
  - Delete snapshot (with confirmation)
  - Navigate between views
  - Filter/search functionality

## Architecture Notes

### Following Existing Patterns

This implementation follows the established patterns in gcon:

1. **GCP Client Layer**: Similar to disk operations in `compute.go`
2. **Table-based List View**: Pattern from `DisksView` with Bubble Tea table
3. **Details View**: Pattern from `DiskDetailsView` with sections
4. **Navigation**: Message-based navigation like `DiskSelectedMsg`
5. **Context Propagation**: Using `ProgramContext` for dimensions and styles

### Key Design Decisions

1. **Read-only by default**: Focus on viewing and deleting, not creating snapshots
2. **Confirmation for delete**: Safety measure for destructive actions
3. **Consistent styling**: Reuse existing styles and symbols
4. **Async operations**: All GCP API calls use `tea.Cmd` for non-blocking execution

## Requirements

- Go 1.22+
- GCP compute API permissions
- Application Default Credentials configured
- Existing project selection

## Testing Approach

- Unit tests for GCP client functions
- View tests for rendering and state transitions
- Table-driven tests where applicable
- Mock GCP API responses for predictable testing

## Success Criteria

- [x] Can list all snapshots in a project
- [x] Can view detailed snapshot information
- [x] Can delete snapshots with confirmation dialog
- [x] All tests pass
- [x] Linter passes
- [x] Navigation works smoothly between views
- [x] Follows existing code style and patterns
