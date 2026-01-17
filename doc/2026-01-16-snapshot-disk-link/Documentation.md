# Feature: Source Disk Link Navigation in Snapshot Details

**Date:** 2026-01-16
**Task ID:** 2026-01-16-snapshot-disk-link
**Branch:** 2026-01-16-snapshot-disk-link

## Summary

Added navigable source disk link in snapshot details view, allowing users to quickly navigate from a snapshot to its source disk details view and return using Esc.

## Changes Made

### 1. Enhanced Snapshot Details View (`internal/ui/views/snapshot_details.go`)

#### Added Components and Types
- Imported `links` component package for navigable links
- Added `diskLink *links.Links` field to `SnapshotDetailsView` struct
- Created `SnapshotDiskSelectedMsg` message type for navigation
- Created `diskLinkData` helper struct to store disk information

#### Key Functions Added
- `extractDiskInfoFromSnapshotSource()`: Parses source disk URL to extract disk name and zone
  - Uses regex pattern: `projects/[^/]+/zones/([^/]+)/disks/([^/]+)`
  - Returns disk name and zone from GCP resource URL

- `populateDiskLink()`: Creates link item from snapshot's source disk
  - Extracts disk info from `SourceDiskID` field
  - Sets up navigable link with disk name and zone data

- `GetComputeClient()`: Returns compute client for view reuse

#### Modified Behavior
- **renderContent()**: Source disk now renders as a navigable link when available
  - Shows cursor indicator (▸) when focused
  - Highlights on focus with background color
  - Falls back to plain text if disk info unavailable

- **updateViewportContent()**: Populates disk link before rendering content

- **Update()**: Enhanced keyboard handling
  - Routes j/k/Enter keys to disk link when available
  - Emits `SnapshotDiskSelectedMsg` when Enter is pressed
  - Updates viewport content on link focus changes

- **View()**: Context-sensitive help text
  - Shows "j/k: select disk • enter: view disk" when disk link is available
  - Falls back to standard scroll help when no link

### 2. App Navigation Handler (`internal/ui/app_navigation.go`)

Added `handleSnapshotDiskSelected()` function:
- Tracks recent disk access for quick navigation
- Pushes current view to navigation stack
- Creates disk details view with proper compute client
- Updates sidebar active view highlight
- Initializes disk details view

### 3. App Message Routing (`internal/ui/app.go`)

Added routing for `SnapshotDiskSelectedMsg`:
- Calls `handleSnapshotDiskSelected()` when message is received
- Enables navigation from snapshot details to disk details

## Technical Details

### Navigation Flow

```mermaid
sequenceDiagram
    participant User
    participant SnapshotDetails
    participant Links
    participant App
    participant DiskDetails

    User->>SnapshotDetails: Press j/k to navigate
    SnapshotDetails->>Links: Update focus
    Links-->>SnapshotDetails: Re-render with highlight

    User->>SnapshotDetails: Press Enter
    SnapshotDetails->>Links: LinkSelectedMsg
    Links->>SnapshotDetails: Extract disk info
    SnapshotDetails->>App: SnapshotDiskSelectedMsg
    App->>App: Push view to stack
    App->>DiskDetails: Navigate to disk details
    DiskDetails-->>User: Show disk details

    User->>App: Press Esc
    App->>App: Pop view from stack
    App->>SnapshotDetails: Return to snapshot details
    SnapshotDetails-->>User: Restore snapshot view
```

### URL Parsing

Source disk URLs follow GCP's standard format:
```
projects/{project-id}/zones/{zone}/disks/{disk-name}
```

The regex pattern extracts:
- Zone (capture group 1): Used for API calls to disk details
- Disk name (capture group 2): Displayed to user and used for navigation

### Link Component Integration

The implementation reuses the existing `links` component pattern from instance details:
1. Initialize `links.New()` in view constructor
2. Populate links in `updateViewportContent()`
3. Check `links.HandleKey()` to determine key routing
4. Call `links.Update()` for navigation handling
5. Use `links.RenderRow()` for consistent link rendering

### Edge Cases Handled

1. **Snapshot without source disk**: Link component set to empty, shows plain text
2. **Invalid source disk URL**: Regex returns empty strings, no link created
3. **Deleted source disk**: Navigation proceeds normally; disk details view will show error
4. **View stack integrity**: Current view pushed before navigation, restored on Esc

## User Experience

### Before
- Source disk shown as plain text field
- No way to navigate to disk details from snapshot
- Required manual navigation: Esc → Compute → Disks → find disk → Enter

### After
- Source disk shown with cursor indicator (▸) when focused
- Press Enter to navigate directly to disk details
- Press Esc to return to snapshot details
- Help text shows available navigation actions

## Testing

### Build and Tests
```bash
go build ./...     # Success
go test ./...      # All tests pass
make lint          # 0 issues
```

### Manual Testing Checklist
- [x] Snapshot with source disk shows navigable link
- [x] j/k navigation highlights the link
- [x] Enter key navigates to disk details
- [x] Disk details loads correctly with proper zone/name
- [x] Esc key returns to snapshot details view
- [x] Help text updates based on link availability
- [x] Snapshot without source disk shows plain text (no crash)

## Files Modified

1. `internal/ui/views/snapshot_details.go` - Enhanced with link navigation
2. `internal/ui/app_navigation.go` - Added disk selection handler
3. `internal/ui/app.go` - Added message routing

## Files Created

1. `doc/2026-01-16-snapshot-disk-link/TODO.md` - Task planning
2. `doc/2026-01-16-snapshot-disk-link/Documentation.md` - This file

## Future Enhancements

Potential improvements for future iterations:
- Add link navigation to snapshot's source image if available
- Show tooltip with full disk URL on hover
- Add breadcrumb trail showing navigation path
- Support keyboard shortcuts (e.g., 'd' for disk) without focusing link first

## Related Patterns

This implementation follows the established navigation pattern used in:
- Instance details → Disk details (via `InstanceDiskSelectedMsg`)
- Disks list → Disk details (via `DiskSelectedMsg`)
- Images list → Image details (via `ImageSelectedMsg`)

The consistent pattern makes the codebase maintainable and predictable for users.
