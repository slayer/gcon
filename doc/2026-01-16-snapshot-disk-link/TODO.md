# Task: Add Source Disk Link Navigation in Snapshot Details View

**Task ID:** 2026-01-16-snapshot-disk-link
**Date:** 2026-01-16

## Description

Add navigable source disk link in snapshot details view that allows users to:
1. View the source disk as a clickable/selectable link (like in VM instance details)
2. Press Enter to navigate to disk details view from snapshot details view
3. Press Esc to return back to snapshot details view

## Current State

- Snapshot details view displays source disk as plain text at line 280 in `snapshot_details.go`
- Instance details view already has working disk link navigation using the `links` component
- Navigation infrastructure is in place via `app_navigation.go`

## Implementation Plan

### 1. Add Links Component to SnapshotDetailsView
- Import the `links` component package
- Add `diskLink *links.Links` field to `SnapshotDetailsView` struct
- Initialize the links component in `NewSnapshotDetailsView`

### 2. Create SnapshotDiskSelectedMsg Message Type
- Define new message type in `snapshot_details.go` similar to `InstanceDiskSelectedMsg`
- Message should contain: DiskName, Zone (extracted from source disk URL)

### 3. Populate Disk Link from Snapshot Details
- Create helper function to extract disk name and zone from source disk URL
- Similar to `extractDiskInfoFromSource` in `instance_details.go`
- Source disk format: `projects/{project}/zones/{zone}/disks/{diskName}`
- Populate link in `updateViewportContent` method

### 4. Render Source Disk as Navigable Link
- Modify `renderContent` method to render source disk using links component
- Show cursor indicator when disk link is focused
- Highlight on focus similar to instance details disk rendering

### 5. Handle Keyboard Navigation
- Route j/k/Enter keys to diskLink component when available
- Check `links.HandleKey()` to determine if key should be routed to links
- Emit `SnapshotDiskSelectedMsg` when Enter is pressed on disk link
- Update help text to show disk navigation hint when link is available

### 6. Add App-Level Navigation Handler
- Add handler for `SnapshotDiskSelectedMsg` in `app_navigation.go`
- Extract disk name and zone from message
- Navigate to disk details view with proper compute client
- Push current view to view stack for back navigation

### 7. Update Message Routing in App
- Add case for `SnapshotDiskSelectedMsg` in app's Update method
- Route to new navigation handler

## Files to Modify

1. `internal/ui/views/snapshot_details.go`
   - Add links component field
   - Add SnapshotDiskSelectedMsg type
   - Add helper to extract disk info from URL
   - Modify renderContent to use links
   - Update key handling
   - Update help text

2. `internal/ui/app_navigation.go`
   - Add `handleSnapshotDiskSelected` function

3. `internal/ui/app.go`
   - Add routing for SnapshotDiskSelectedMsg

## Testing Strategy

1. Manual testing:
   - Create a snapshot from a disk
   - Navigate to snapshot details
   - Verify source disk shows with cursor indicator
   - Test j/k navigation (if multiple items in future)
   - Press Enter to navigate to disk details
   - Verify disk details load correctly
   - Press Esc to return to snapshot details
   - Verify we're back at snapshot details view

2. Edge cases:
   - Snapshot without source disk (deleted disk)
   - Snapshot with source disk in different zone
   - Navigation stack integrity

## Constraints

- Must follow existing navigation pattern from instance details
- Must preserve scroll position when returning via Esc
- Must handle case where source disk no longer exists gracefully
- Must show clear visual feedback for navigable link

## Progress

- [ ] Add links component to SnapshotDetailsView struct
- [ ] Define SnapshotDiskSelectedMsg message type
- [ ] Create helper to extract disk name/zone from source URL
- [ ] Populate disk link in updateViewportContent
- [ ] Modify renderContent to render disk as link
- [ ] Add keyboard navigation handling
- [ ] Update help text for disk navigation
- [ ] Add app-level navigation handler
- [ ] Update app message routing
- [ ] Test navigation flow
- [ ] Test edge cases
- [ ] Run tests and linter
