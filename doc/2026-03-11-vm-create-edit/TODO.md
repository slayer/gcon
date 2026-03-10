# VM Instance Create/Edit

## Task Description

Add ability to create new VM instances and edit existing ones via form-based UI.

**Tier 1 (this task):** Name, zone, machine type, boot disk (image + size + type), networking (network, subnet, external IP).

**Tier 2 (future):** Labels, tags, service account, startup script, deletion protection, scheduling (preemptible/spot), additional disks, metadata.

## Implementation Plan

### Phase 1: GCP Client Layer
- [ ] Add `InstanceCreateConfig` struct
- [ ] Add `MachineType`, `NetworkInfo`, `SubnetworkInfo` types
- [ ] Add `CreateInstance()` method
- [ ] Add `SetMachineType()` method
- [ ] Add `ResizeBootDisk()` method
- [ ] Add `ListMachineTypes()` method (zone-specific)
- [ ] Add `ListSubnetworks()` method (region-specific)
- [ ] Add curated boot disk images list
- [ ] Add curated disk type options
- [ ] Write tests for new client methods

### Phase 2: Shared Form Builder
- [ ] Create `instance_form.go` with shared form-building logic
- [ ] `buildInstanceForm(mode)` — creates form with all sections
- [ ] `populateFromDetails(form, details)` — fills form from existing instance
- [ ] Machine type caching (`map[string][]MachineType` by zone)
- [ ] Zone change triggers async machine type fetch
- [ ] Network/subnetwork dynamic loading

### Phase 3: Create View
- [ ] Create `instance_create.go` using `CreateViewBase`
- [ ] Form sections: Basic, Machine Config, Boot Disk, Networking
- [ ] Validation: resource name, disk size, required fields
- [ ] Submit flow: form → saving → result
- [ ] Add `c` key binding to instances list view
- [ ] Add message types: `InstanceCreateRequestMsg`, `CreateInstanceMsg`, `InstanceCreateResultMsg`
- [ ] Write view tests

### Phase 4: Edit View
- [ ] Create `instance_edit.go` with state machine (Loading → Form → Diff → Saving)
- [ ] Load current instance config on init
- [ ] Read-only fields: name, zone, boot disk image, disk type, network, subnetwork
- [ ] Editable fields: machine type, disk size (expand only), external IP (defer to tier 2)
- [ ] Diff preview before applying changes
- [ ] Sequential API calls: SetMachineType, ResizeBootDisk
- [ ] Partial failure reporting
- [ ] Machine type warning when instance is running
- [ ] Add `e` key binding to instance details view
- [ ] Add message types: `InstanceEditRequestMsg`, `InstanceEditSubmitMsg`, `InstanceEditResultMsg`
- [ ] Write view tests

### Phase 5: App Integration
- [ ] Add `ViewInstanceCreate` and `ViewInstanceEdit` to ViewType enum
- [ ] Add view fields to App struct
- [ ] Add cases to `getCurrentViewModel()`
- [ ] Add cases to `renderCurrentView()`
- [ ] Add message handlers in `Update()`
- [ ] Add navigation handlers in `app_navigation.go`
- [ ] Add to `clearAllViews()`
- [ ] Add to `updateViewSizes()`
- [ ] Update sidebar guards for Compute Engine hierarchy
- [ ] Add command palette entry: "Compute: Create Instance"
- [ ] Update key bindings doc

### Phase 6: Testing & Polish
- [ ] Run full test suite
- [ ] Run linter
- [ ] Manual testing: create instance, edit instance
- [ ] Verify error handling (API failures, validation)
- [ ] Verify cancel during saving works
- [ ] Update CLAUDE.md implemented features list
