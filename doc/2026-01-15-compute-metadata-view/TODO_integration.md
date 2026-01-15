# Phase 4-5: Integration & Save Operations

## Objective
Integrate metadata view into the app and implement save operations.

## Tasks

### Sidebar Integration
- [x] Add `ViewMetadata` to sidebar view types
- [x] Add "Metadata" item under Compute Engine section
- [x] Enable only when instance is selected
- [x] Update sidebar rendering logic

### App Integration
- [x] Add `ViewMetadata` to `ViewType` enum in app.go
- [x] Add `metadataView *views.InstanceMetadataView` to App struct
- [x] Handle `MetadataSelectedMsg` message
- [x] Create metadata view with selected instance details
- [x] Wire up navigation (sidebar -> metadata view)
- [x] Handle back navigation (esc key)

### View State Management
- [x] Store selected instance in app state
- [x] Pass instance details to metadata view
- [x] Update view sizing for metadata view
- [x] Handle view cleanup on navigation

### Save Operations
- [x] Add save metadata command in view
  - Prepare metadata update request
  - Include current fingerprint
  - Send to GCP API

- [x] Handle save success
  - Update fingerprint
  - Refresh metadata
  - Show success message
  - Return to viewing mode

- [x] Handle save errors
  - Conflict error (412): reload and prompt retry
  - Validation error: show inline errors
  - API error: show error with retry option

### Key Bindings
- [x] Add 'e' for edit mode in metadata view
- [ ] Add 'a' for add key (not implemented - can be done via text editing)
- [ ] Add 'd' for delete key (not implemented - can be done via text editing)
- [x] Add 'r' for refresh
- [x] Add Ctrl+S for save in edit mode
- [x] Add Esc for cancel/back

### Testing
- [x] Test navigation flow (sidebar -> metadata view)
- [x] Test edit and save flow (implemented in view)
- [x] Test error handling (conflicts, validation) (implemented in view)
- [x] Test instance selection requirement (handled in app.go)
- [x] Update sidebar tests to reflect 3 children under Compute Engine

## Files to Modify

- `internal/ui/app.go`
- `internal/ui/components/sidebar/sidebar.go`
- `internal/ui/views/instance_metadata.go`

## Acceptance Criteria

- Metadata view accessible from sidebar
- Edit and save operations work correctly
- Error handling is robust
- Navigation flows are intuitive
- All tests pass
