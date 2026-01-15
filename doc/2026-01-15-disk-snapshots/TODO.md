# Task: Disk Snapshots Management

## Task Description

Implement disk snapshots management features in gcon:
- List all snapshots across all zones
- View detailed snapshot information
- Delete snapshots with confirmation

## Implementation Plan

### 1. GCP API Client Layer (`internal/gcp/compute.go`)

- [ ] Add `Snapshot` struct with essential fields
- [ ] Add `SnapshotDetails` struct with comprehensive information
- [ ] Implement `ListSnapshots(ctx, projectID)` - list all snapshots
- [ ] Implement `GetSnapshotDetails(ctx, projectID, snapshotName)` - get snapshot details
- [ ] Implement `DeleteSnapshot(ctx, projectID, snapshotName)` - delete a snapshot
- [ ] Add tests for snapshot client functions

### 2. Snapshots List View (`internal/ui/views/snapshots.go`)

- [ ] Create `SnapshotsView` struct following the pattern from `DisksView`
- [ ] Implement table display with columns:
  - Name (with status indicator)
  - Source Disk
  - Size (GB)
  - Created At
  - Status
- [ ] Add key bindings:
  - `enter` - view snapshot details
  - `r` - refresh list
  - `/` - filter/search
  - `D` - delete snapshot (with confirmation)
- [ ] Handle loading/error states
- [ ] Add tests for snapshots view

### 3. Snapshot Details View (`internal/ui/views/snapshot_details.go`)

- [ ] Create `SnapshotDetailsView` struct following the pattern from `DiskDetailsView`
- [ ] Display comprehensive snapshot information:
  - Basic: Name, ID, Description, Status
  - Source: Source disk name and zone
  - Size: Total size, storage bytes
  - Timestamps: Created, Last used
  - Storage location
  - Labels
  - Encryption info
- [ ] Add key bindings:
  - `D` - delete snapshot (with confirmation)
  - `esc` - back to snapshots list
- [ ] Add tests for snapshot details view

### 4. Integration & Navigation

- [ ] Add `ViewSnapshots` and `ViewSnapshotDetails` to `ViewType` enum in `app.go`
- [ ] Add snapshot views to `App` struct
- [ ] Handle `SnapshotSelectedMsg` navigation message
- [ ] Add "Snapshots" item to sidebar menu
- [ ] Update command palette to include snapshot navigation
- [ ] Handle back navigation (esc key)

### 5. Testing & Quality

- [ ] Run tests for all new code
- [ ] Run full test suite to catch regressions
- [ ] Run linter (`make lint`)
- [ ] Manual testing:
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

- [ ] Can list all snapshots in a project
- [ ] Can view detailed snapshot information
- [ ] Can delete snapshots with confirmation dialog
- [ ] All tests pass
- [ ] Linter passes
- [ ] Navigation works smoothly between views
- [ ] Follows existing code style and patterns
