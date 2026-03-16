# VM Instance Create/Edit

## Task Description

Add ability to create new VM instances and edit existing ones via form-based UI.

**Tier 1 (this task):** Name, zone, machine type, boot disk (image + size + type), networking (network, subnet, external IP).

**Tier 2 (future):** Labels, tags, service account, startup script, deletion protection, scheduling (preemptible/spot), additional disks, metadata.

## Implementation Plan

### Phase 1: GCP Client Layer
- [x] Add `InstanceCreateConfig` struct
- [x] Add `MachineType`, `NetworkInfo`, `SubnetworkInfo` types
- [x] Add `CreateInstance()` method
- [x] Add `SetMachineType()` method
- [x] Add `ResizeBootDisk()` method
- [x] Add `ListMachineTypes()` method (zone-specific)
- [x] Add `ListSubnetworks()` method (region-specific)
- [x] Add curated boot disk images list
- [x] Add curated disk type options
- [x] Write tests for new client methods

### Phase 2: Shared Form Builder
- [x] Create `instance_form.go` with shared form-building logic
- [x] `buildInstanceForm(mode)` — creates form with all sections
- [x] `populateFromDetails(form, details)` — fills form from existing instance
- [x] Machine type caching (`map[string][]MachineType` by zone)
- [x] Zone change triggers async machine type fetch
- [x] Network/subnetwork dynamic loading

### Phase 3: Create View
- [x] Create `instance_create.go` using `CreateViewBase`
- [x] Form sections: Basic, Machine Config, Boot Disk, Networking
- [x] Validation: resource name, disk size, required fields
- [x] Submit flow: form → saving → result
- [x] Add `c` key binding to instances list view
- [x] Add message types: `InstanceCreateRequestMsg`, `CreateInstanceMsg`, `InstanceCreateResultMsg`
- [x] Write view tests

### Phase 4: Edit View
- [x] Create `instance_config_edit.go` with state machine (Loading → Form → Diff → Saving)
- [x] Load current instance config on init
- [x] Read-only fields: name, zone, boot disk image, disk type, network, subnetwork
- [x] Editable fields: machine type, disk size (expand only)
- [x] Diff preview before applying changes
- [x] Sequential API calls: SetMachineType, ResizeBootDisk
- [x] Partial failure reporting
- [x] Machine type warning when instance is running
- [x] Add `e` key binding to instance details view
- [x] Add message types: `InstanceConfigEditRequestMsg`, `InstanceConfigEditSubmitMsg`, `InstanceConfigEditResultMsg`
- [x] Write view tests

### Phase 5: App Integration
- [x] Add `ViewInstanceCreate` and `ViewInstanceConfigEdit` to ViewType enum
- [x] Add view fields to App struct
- [x] Add cases to `getCurrentViewModel()`
- [x] Add cases to `renderCurrentView()`
- [x] Add message handlers in `Update()`
- [x] Add navigation handlers in `app_navigation.go`
- [x] Add to `clearAllViews()`
- [x] Add to `updateViewSizes()`
- [x] Update sidebar guards for Compute Engine hierarchy
- [x] Update `updateSidebarActiveView()` for new views
- [x] Add breadcrumb support for create/edit views
- [x] Add Esc/back cleanup in leavingView switch
- [x] Add to `reloadCurrentView()` for project switching
- [ ] Add command palette entry: "Compute: Create Instance" (skipped — accessible via `c` key from instances list)
- [ ] Update key bindings doc

### Phase 6: Testing & Polish
- [x] Run full test suite
- [x] Run linter
- [ ] Manual testing: create instance, edit instance
- [ ] Verify error handling (API failures, validation)
- [ ] Verify cancel during saving works
- [ ] Update CLAUDE.md implemented features list
