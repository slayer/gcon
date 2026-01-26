# Resource Editing & Creation - TODO

**Status:** Design Complete, Ready for Implementation
**Started:** 2026-01-18

---

## Phase 1: Core Form Framework ⏳
**Branch:** `2026-01-18-form-framework`
**Duration:** 1 week
**Status:** ~30% complete (DiffViewer done, forms framework not started)

### Package Structure
- [ ] Create `internal/ui/components/forms/` directory
- [ ] Create `internal/ui/components/forms/types.go`
- [ ] Create `internal/ui/components/forms/field.go`
- [ ] Create `internal/ui/components/forms/field_test.go`
- [ ] Create `internal/ui/components/forms/section.go`
- [ ] Create `internal/ui/components/forms/section_test.go`
- [ ] Create `internal/ui/components/forms/form.go`
- [ ] Create `internal/ui/components/forms/form_test.go`
- [ ] Create `internal/ui/components/forms/diff.go`
- [ ] Create `internal/ui/components/forms/diff_test.go`
- [ ] Create `internal/ui/components/forms/validators.go`
- [ ] Create `internal/ui/components/forms/validators_test.go`

### FormField Component
- [ ] Define field types enum (Text, Number, Dropdown, MultiSelect, Toggle, ReadOnly)
- [ ] Implement FormField struct
- [ ] Implement `Init()` method
- [ ] Implement `Update()` method
- [ ] Implement `View()` method for each field type
- [ ] Implement focus management
- [ ] Implement validation triggering
- [ ] Add keyboard navigation (Tab, Enter, Arrows)
- [ ] Add help text rendering
- [ ] Write comprehensive tests (each field type, validation, focus)

### FormSection Component
- [ ] Implement FormSection struct
- [ ] Implement field grouping
- [ ] Implement collapsible behavior
- [ ] Implement focus navigation between fields
- [ ] Implement section header rendering
- [ ] Write tests (navigation, collapse, rendering)

### FormView Component
- [ ] Implement FormView struct
- [ ] Implement section management
- [ ] Implement global navigation (Tab/Shift+Tab)
- [ ] Implement form-level validation
- [ ] Implement submit/cancel handling
- [ ] Implement keyboard shortcuts
- [ ] Implement responsive layout
- [ ] Add viewport scrolling for long forms
- [ ] Write tests (navigation, validation, data collection)

### DiffViewer Component ✅
*Implemented in `internal/ui/components/diff/` as part of Instance Labels Editor*
- [x] Implement DiffViewer struct (`internal/ui/components/diff/diff.go`)
- [x] Implement diff field rendering (before/after)
- [x] Implement changed field highlighting (colors)
- [ ] Implement warning display
- [ ] Implement cost impact display
- [x] Implement Yes/No confirmation
- [x] Write tests (rendering, selection) (`internal/ui/components/diff/diff_test.go`)

### Validators
- [ ] Implement `ValidateRequired()`
- [ ] Implement `ValidateGCPResourceName()`
- [ ] Implement `ValidateNumber()` with min/max
- [ ] Implement `ValidateStringLength()`
- [ ] Implement `ValidatePattern()` with regex
- [ ] Implement `ComposeValidators()` for chaining
- [ ] Write tests for all validators

### Demo View
- [ ] Create `internal/ui/views/form_demo.go`
- [ ] Add sample form with all field types
- [ ] Add validation examples
- [ ] Add diff viewer example
- [ ] Register in command palette

### Documentation
- [ ] Document form components API
- [ ] Add usage examples
- [ ] Update CLAUDE.md

### Phase 1 Complete
- [ ] All tests pass
- [ ] Code linted
- [ ] Demo works
- [ ] Commit and push to branch

---

## Phase 2: Disk Editing - Resize ⏸️
**Branch:** `2026-01-18-disk-resize`
**Duration:** 1 week

### GCP API
- [ ] Add `ResizeDisk()` method to `internal/gcp/compute.go`
- [ ] Implement async operation waiting
- [ ] Add error handling
- [ ] Write tests with mock client

### DiskEditor Component
- [ ] Create `internal/ui/views/disk_editor.go`
- [ ] Create `internal/ui/views/disk_editor_test.go`
- [ ] Define DiskEditor struct
- [ ] Implement `buildResizeForm()`
- [ ] Implement state machine (form, diff, saving, done)
- [ ] Implement `Update()` logic
- [ ] Implement `View()` rendering
- [ ] Implement diff generation
- [ ] Implement save command
- [ ] Write tests

### View Integration
- [ ] Add "Resize Disk" to disks view action menu
- [ ] Add 'r' key binding to disks view
- [ ] Update `internal/ui/views/disks.go`

### Messages
- [ ] Add `resizeRequestMsg` to `internal/ui/messages.go`
- [ ] Add `EditRequestMsg`, `EditCompleteMsg`, `EditCancelledMsg`

### App Integration
- [ ] Add diskEditor field to App struct
- [ ] Handle `resizeRequestMsg` in App.Update
- [ ] Handle `EditCompleteMsg` in App.Update
- [ ] Handle `EditCancelledMsg` in App.Update

### Documentation
- [ ] Update CLAUDE.md with disk resize feature
- [ ] Update README.md with 'r' key binding
- [ ] Document in TODO.md

### Phase 2 Complete
- [ ] Manual test: resize disk in gcon
- [ ] Verify in GCP console
- [ ] All tests pass
- [ ] Commit and push

---

## Phase 3: Snapshot Creation ⏸️
**Branch:** `2026-01-18-snapshot-create`
**Duration:** 1 week

### GCP API
- [ ] Add `CreateSnapshot()` method to `internal/gcp/compute.go`
- [ ] Implement async operation waiting
- [ ] Add error handling
- [ ] Write tests

### SnapshotEditor Component
- [ ] Create `internal/ui/views/snapshot_editor.go`
- [ ] Create `internal/ui/views/snapshot_editor_test.go`
- [ ] Define SnapshotEditor struct
- [ ] Implement `buildCreateForm()`
- [ ] Implement state machine
- [ ] Implement Update and View
- [ ] Write tests

### View Integration
- [ ] Add "Create Snapshot" to disks view action menu
- [ ] Add 's' key binding to disks view
- [ ] Add "Create Snapshot" to disk details view

### Messages
- [ ] Add `createSnapshotRequestMsg` to messages.go

### App Integration
- [ ] Handle snapshot creation in App.Update

### Documentation
- [ ] Update docs

### Phase 3 Complete
- [ ] Manual test
- [ ] All tests pass
- [ ] Commit and push

---

## Phase 4: Disk Creation ⏸️
**Branch:** `2026-01-18-disk-create`
**Duration:** 1 week

### GCP API
- [ ] Add `CreateDisk()` method
- [ ] Add `ListOSImages()` method
- [ ] Write tests

### Image Selector Component
- [ ] Create `internal/ui/components/image_selector.go`
- [ ] Implement searchable image list
- [ ] Group by OS family
- [ ] Add selection UI

### DiskEditor Enhancement
- [ ] Add "from snapshot" mode
- [ ] Add "from image" mode with image selector
- [ ] Add "blank disk" mode
- [ ] Implement form for each mode

### View Integration
- [ ] Add "Create Disk" to disks view (key 'n')
- [ ] Add "Create Disk from Snapshot" to snapshots view
- [ ] Add "Create Disk from Image" to images view

### App Integration
- [ ] Handle disk creation modes

### Documentation
- [ ] Update docs

### Phase 4 Complete
- [ ] Test all three creation paths
- [ ] All tests pass
- [ ] Commit and push

---

## Phase 5: Instance Labels Editing ✅ COMPLETE
**Branch:** `2026-01-18-resource-editing`
**Duration:** 1 week
**Completed:** 2026-01-26

### GCP API ✅
- [x] Add `SetInstanceLabels()` method (`internal/gcp/compute_labels.go`)
- [x] Add `GetInstanceLabelsFingerprint()` method (`internal/gcp/compute_labels.go`)
- [x] Handle fingerprint for optimistic concurrency
- [x] Write tests (`internal/gcp/compute_labels_test.go`)

### InstanceEditor Component ✅
- [x] Create `internal/ui/views/instance_editor.go`
- [x] Create `internal/ui/views/instance_editor_test.go`
- [x] Define InstanceEditor struct
- [x] Implement state machine (loading, form, diff, saving, error)
- [x] Write tests

### LabelEditor Component ✅
*Dedicated label editing component*
- [x] Create `internal/ui/components/labeledit/labeledit.go`
- [x] Create `internal/ui/components/labeledit/labeledit_test.go`
- [x] Add/edit/delete labels functionality
- [x] Key/value input with Tab switching
- [x] GCP label validation (63 chars, lowercase, etc.)

### View Integration ✅
- [x] Add "Edit Labels" to instance details action menu
- [x] Add 'l' key binding for quick access

### App Integration ✅
- [x] Add ViewInstanceEditor to app views
- [x] Handle InstanceEditorMsg in App.Update
- [x] Handle navigation back from editor

### Documentation ✅
- [x] Update README.md with key bindings
- [x] Update CLAUDE.md with key bindings

### Phase 5 Complete ✅
- [x] Manual test
- [x] All tests pass
- [x] Code committed

---

## Phase 6: Instance Machine Type Editing ⏸️
**Branch:** `2026-01-18-instance-machine-type`
**Duration:** 1.5 weeks

### GCP API
- [ ] Add `SetMachineType()` method
- [ ] Add `ListMachineTypes()` method
- [ ] Write tests

### InstanceEditor Enhancement
- [ ] Implement `buildMachineTypeForm()`
- [ ] Add machine family selector
- [ ] Add machine type selector (filtered by family)
- [ ] Show vCPU/memory specs

### Multi-Step Operation
- [ ] Implement stop-change-restart workflow
- [ ] Add progress tracking
- [ ] Show status for each step
- [ ] Add rollback on failure

### Diff Viewer Enhancement
- [ ] Add restart warning
- [ ] Add downtime estimate

### View Integration
- [ ] Add "Change Machine Type" to action menu

### Documentation
- [ ] Update docs

### Phase 6 Complete
- [ ] Manual test with running instance
- [ ] Test error mid-operation
- [ ] All tests pass
- [ ] Commit and push

---

## Phase 7: CreateWizard Component ⏸️
**Branch:** `2026-01-18-wizard-component`
**Duration:** 1 week

### Wizard Framework
- [ ] Create `internal/ui/components/forms/wizard.go`
- [ ] Create `internal/ui/components/forms/wizard_test.go`
- [ ] Define WizardStep struct
- [ ] Define CreateWizard struct
- [ ] Implement step navigation (Next/Back)
- [ ] Implement step indicator (1/5, 2/5, etc.)
- [ ] Implement per-step validation
- [ ] Implement data accumulation
- [ ] Implement review step

### Tests
- [ ] Test navigation
- [ ] Test validation blocking
- [ ] Test data collection
- [ ] Test skip optional steps

### Demo
- [ ] Add wizard example to form demo

### Documentation
- [ ] Document wizard API

### Phase 7 Complete
- [ ] All tests pass
- [ ] Commit and push

---

## Phase 8: Instance Creation - Clone ⏸️
**Branch:** `2026-01-18-instance-clone`
**Duration:** 1 week

### GCP API
- [ ] Add `CreateInstance()` method
- [ ] Implement full instance config building
- [ ] Write tests

### InstanceEditor Enhancement
- [ ] Implement `buildCloneForm()`
- [ ] Implement `generateUniqueName()`
- [ ] Copy all settings from source instance
- [ ] Allow modifications before creation

### View Integration
- [ ] Add "Clone Instance" to instances view (key 'c')
- [ ] Add choice modal (clone/template/scratch)

### App Integration
- [ ] Handle instance cloning

### Documentation
- [ ] Update docs

### Phase 8 Complete
- [ ] Manual test cloning
- [ ] Verify all settings copied
- [ ] All tests pass
- [ ] Commit and push

---

## Phase 9: Instance Creation - Templates ⏸️
**Branch:** `2026-01-18-instance-templates`
**Duration:** 1 week

### Template System
- [ ] Create `internal/templates/instances.go`
- [ ] Create `internal/templates/instances_test.go`
- [ ] Define InstanceTemplate struct
- [ ] Create 3-4 predefined templates (web, db, dev, custom)

### Template Selector
- [ ] Implement template selection UI
- [ ] Show template descriptions
- [ ] Show template details on focus

### InstanceEditor Enhancement
- [ ] Implement template-based form
- [ ] Allow template customization

### View Integration
- [ ] Add "From Template" to creation choice modal

### App Integration
- [ ] Handle template-based creation

### Documentation
- [ ] Document templates

### Phase 9 Complete
- [ ] Test all templates
- [ ] All tests pass
- [ ] Commit and push

---

## Phase 10: Instance Creation - Full Wizard ⏸️
**Branch:** `2026-01-18-instance-wizard`
**Duration:** 2 weeks

### Image Browser
- [ ] Create `internal/ui/components/image_browser.go`
- [ ] List images grouped by OS family
- [ ] Add search/filter
- [ ] Show image details

### Wizard Implementation
- [ ] Step 1: Basic Info form
- [ ] Step 2: Machine Type form
- [ ] Step 3: Boot Disk form with image browser
- [ ] Step 4: Network form
- [ ] Step 5: Review step with full config

### InstanceEditor Enhancement
- [ ] Implement `buildCreateWizard()`
- [ ] Wire up 5 steps
- [ ] Implement review rendering

### View Integration
- [ ] Add "From Scratch" to creation choice modal

### App Integration
- [ ] Handle wizard-based creation

### Documentation
- [ ] Update docs

### Phase 10 Complete
- [ ] Test full wizard flow
- [ ] Test navigation (Next/Back)
- [ ] Test validation at each step
- [ ] All tests pass
- [ ] Commit and push

---

## Phase 11: Polish & Advanced Features ⏸️
**Branch:** `2026-01-18-editing-polish`
**Duration:** 1 week

### Cost Estimation
- [ ] Research GCP pricing API or use hardcoded estimates
- [ ] Add cost calculation to editors
- [ ] Show hourly and monthly estimates in diff

### Template Saving
- [ ] Implement template serialization
- [ ] Save to `~/.gcon/templates.json`
- [ ] Load user templates at startup
- [ ] Add "Save as Template" option

### Form Field Search
- [ ] Add search to long dropdowns (100+ options)
- [ ] Implement type-to-filter
- [ ] Highlight matching text

### Keyboard Shortcuts Help
- [ ] Create help overlay for forms
- [ ] Show all shortcuts
- [ ] Make context-sensitive

### Auto-Save
- [ ] Implement form state persistence
- [ ] Save to temp file periodically
- [ ] Prompt to recover on restart

### Performance Optimization
- [ ] Lazy-load dropdown options
- [ ] Debounce validation (300ms)
- [ ] Optimize re-renders
- [ ] Test with large datasets

### Documentation
- [ ] Update CLAUDE.md with all features
- [ ] Update README.md with key bindings
- [ ] Create user guide
- [ ] Add screenshots/GIFs

### Phase 11 Complete
- [ ] All features polished
- [ ] Performance acceptable
- [ ] Documentation complete
- [ ] All tests pass
- [ ] Final lint check
- [ ] Commit and push

---

## Final Checklist

### Code Quality
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] Code coverage > 80%
- [ ] Linter passes with no warnings
- [ ] Code formatted with `go fmt`

### Manual Testing
- [ ] Resize disk workflow
- [ ] Create snapshot workflow
- [ ] Create disk (all 3 paths)
- [ ] Edit instance labels
- [ ] Change instance machine type
- [ ] Clone instance
- [ ] Create from template
- [ ] Create from scratch (full wizard)
- [ ] Test error cases (network, permissions, validation)
- [ ] Test cancel at each step
- [ ] Test keyboard navigation

### Documentation
- [ ] CLAUDE.md updated with all features
- [ ] README.md updated with key bindings
- [ ] User guide created
- [ ] Implementation plan completed
- [ ] Patterns documented
- [ ] API docs updated

### Release
- [ ] Create pull request
- [ ] Code review
- [ ] Merge to main
- [ ] Tag release
- [ ] Announce features
- [ ] Monitor for issues

---

## Progress Tracking

**Phase 1:** ⏳ ~30% (DiffViewer done, forms framework not started)
**Phase 2:** ⏸️ Not Started
**Phase 3:** ⏸️ Not Started
**Phase 4:** ⏸️ Not Started
**Phase 5:** ✅ Complete
**Phase 6:** ⏸️ Not Started
**Phase 7:** ⏸️ Not Started
**Phase 8:** ⏸️ Not Started
**Phase 9:** ⏸️ Not Started
**Phase 10:** ⏸️ Not Started
**Phase 11:** ⏸️ Not Started

**Overall Progress:** ~15% (1.3/11 phases complete)

---

## Notes

### Decisions Made
- Hybrid UX approach (forms for simple, wizard for complex)
- Clone-first workflow
- Diff preview with confirmation
- Action menu + key bindings for triggering

### Open Questions
- Template storage: local first, sync later
- Undo mechanism: Phase 11 feature
- Auto-complete: yes for long dropdowns
- Multi-project editing: not in MVP
- Offline mode: no, require connection

### Risks
- Form framework complexity → mitigation: start minimal, iterate
- GCP API rate limits → mitigation: caching, delays
- State management complexity → mitigation: clear message types

---

## Revision History

| Date       | Version | Changes                           |
|------------|---------|-----------------------------------|
| 2026-01-18 | 1.0     | Initial TODO created              |
| 2026-01-26 | 1.1     | Phase 5 complete, Phase 1 DiffViewer done |
