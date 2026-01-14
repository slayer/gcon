# Disk Details Feature

## Task Description
Add a detail view for Compute Engine disks. Users can press Enter on a disk in the list to see detailed information.

## Implementation Plan

### 1. GCP Client Layer
- [x] Add `DiskDetails` struct to `internal/gcp/compute.go`
- [x] Add `GetDiskDetails()` method to fetch full disk info

### 2. UI View Layer
- [x] Create `internal/ui/views/disk_details.go` with:
  - [x] `DiskDetailsView` struct following instance_details.go pattern
  - [x] Viewport for scrollable content
  - [x] Display: name, zone, size, type, status, source image, encryption, users
  - [x] Key bindings: scroll, refresh, back

### 3. Navigation
- [x] Add `DiskSelectedMsg` to disk_details.go
- [x] Handle Enter key to emit DiskSelectedMsg in DisksView
- [x] Wire up DiskDetailsView in app.go

### 4. Testing
- [x] Add unit tests for diskDetailsFromAPI()
- [x] Add tests for DiskDetailsView

## Requirements
- Show comprehensive disk information
- Scrollable viewport for long content
- Refresh support
- Follow existing patterns from instance_details.go
