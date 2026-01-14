# List Disks Feature

## Task Description
Add a new view to list Compute Engine disks in the terminal UI.

## Implementation Plan

### 1. GCP Client Layer
- [x] Add `Disk` struct to `internal/gcp/compute.go`
- [x] Add `ListDisks()` method using aggregated list API
- [x] Add helper methods for disk status and formatting

### 2. UI View Layer
- [x] Create `internal/ui/views/disks.go` with:
  - [x] `DisksView` struct following instances.go pattern
  - [x] Table columns: Name, Zone, Size, Type, Attached To
  - [x] Key bindings: refresh, filter, back
  - [x] Loading state with spinner
  - [x] Error handling

### 3. App Integration
- [x] Wire up `DisksView` in `internal/ui/app.go`
  - [x] Add `disksView` field to App struct
  - [x] Handle navigation in `handleSidebarNavigation()`
  - [x] Add to `renderCurrentView()`
  - [x] Handle back navigation
  - [x] Handle resize in `updateViewSizes()`

### 4. Testing
- [x] Add unit tests for `diskFromAPI()` and disk methods
- [x] Add tests for `DisksView` rendering

## Requirements
- Show disks across all zones (use AggregatedList)
- Display status indicator (attached/available/etc)
- Support filtering via `/` key
- Follow existing code patterns from instances view
