# GCP Compute Actions Implementation

## Task Overview
Implement Priority 1-3 features for VM instances, images, and snapshots:
- Delete actions for snapshots, images, instances
- Creation actions: disk from snapshot/image, image from snapshot
- Instance lifecycle: suspend/resume, change machine type

## Progress

### Task 1: Snapshot Delete
- [ ] Add `deleteSnapshot` key binding to snapshots view
- [ ] Show confirmation dialog
- [ ] Call `DeleteSnapshot` API
- [ ] Refresh list on success
- [ ] Add to action menu

### Task 2: Image Delete
- [ ] Add `deleteImage` key binding to images view
- [ ] Show confirmation dialog
- [ ] Call `DeleteImage` API
- [ ] Refresh list on success
- [ ] Add to action menu

### Task 3: Instance Delete
- [ ] Add `DeleteInstance` to compute client
- [ ] Create "type name to confirm" dialog variant
- [ ] Check `DeletionProtection` flag
- [ ] Add to action menu

### Task 4: Create Disk from Snapshot
- [x] Add `DiskCreateConfig` struct to compute.go
- [x] Add `CreateDiskFromSnapshot` method to compute.go
- [x] Create `internal/ui/views/disk_create.go` with form
- [x] Add `c` key and action menu option to snapshots.go
- [x] Add `c` key and action menu option to snapshot_details.go
- [x] Add `ViewDiskCreate` view type to app.go
- [x] Add message handlers in app.go (DiskCreateFromSnapshotRequestMsg, DiskCreateCanceledMsg, CreateDiskFromSnapshotMsg)
- [x] Add handlers in app_navigation.go (handleDiskCreateFromSnapshotRequest, handleDiskCreateCanceled, handleCreateDiskFromSnapshot)
- [x] Add ViewDiskCreate case to app_render.go renderCurrentView()
- [x] Add diskCreateView field to App struct
- [x] Add diskCreateView to getCurrentViewModel()
- [x] Add diskCreateView to updateViewSizes()
- [x] Add diskCreateView to clearAllViews()
- [x] Update key-bindings.md documentation
- [x] Run tests and lint (all pass)

### Task 5: Create Disk from Image
- [ ] Add `CreateDiskFromImage` API
- [ ] Reuse disk creation form
- [ ] Add `c` key binding

### Task 6: Create Image from Snapshot
- [ ] Add `CreateImageFromSnapshot` API
- [ ] Add `i` key binding

### Task 7: Instance Suspend/Resume
- [ ] Add `SuspendInstance`, `ResumeInstance` APIs
- [ ] Add `z` key for suspend
- [ ] Add `Z` key for resume
- [ ] Update status display

### Task 8: Change Machine Type
- [ ] Add `SetMachineType` API
- [ ] Create machine type selector
- [ ] Add `m` key binding
- [ ] Only available when STOPPED
