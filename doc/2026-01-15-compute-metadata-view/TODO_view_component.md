# Phase 2: View Component

## Objective
Create the metadata view UI component for displaying and managing instance metadata.

## Tasks

- [x] Create `InstanceMetadataView` struct
  - Store instance details (project, zone, instance name)
  - Store metadata and fingerprint
  - Store project metadata (read-only)
  - Manage view state (loading, viewing, editing)
  - Handle viewport for scrolling

- [x] Implement `Init()` method
  - Start loading metadata
  - Start spinner animation

- [x] Implement `Update()` method
  - Handle metadata loaded message
  - Handle metadata error message
  - Handle metadata saved message
  - Handle key bindings (j/k navigation, e for edit, ctrl+s for save, etc.)
  - Handle spinner tick
  - Update viewport on scroll
  - Forward keys to editor in edit mode

- [x] Implement `View()` method
  - Render loading state with spinner
  - Render saving state
  - Render metadata sections:
    - Custom Metadata
    - SSH Keys (Project) - read-only
    - SSH Keys (Instance)
  - Render help text
  - Handle empty states
  - Render edit mode with metadata editor
  - Show success message after save

- [x] Implement `SetContext()` method
  - Accept shared program context
  - Set viewport dimensions

- [x] Add metadata loading command
  - Fetch instance metadata
  - Fetch project metadata
  - Parse SSH keys
  - Return loaded message

- [x] Add metadata saving command
  - Parse editor content
  - Validate metadata
  - Save to GCP with fingerprint
  - Return saved message

- [x] Add navigation helpers
  - Enter edit mode
  - Exit edit mode
  - Get custom metadata (exclude SSH keys)
  - Parse project SSH keys
  - Parse instance SSH keys

- [x] Create view tests
  - Test rendering in different states
  - Test navigation
  - Test message handling
  - Test edit mode
  - Test metadata parsing
  - Test key bindings

## Files to Create/Modify

- `internal/ui/views/instance_metadata.go` (new)
- `internal/ui/views/instance_metadata_test.go` (new)
- `internal/ui/views/view.go` - May need message types

## Acceptance Criteria

- [x] View displays metadata correctly
- [x] Navigation works (j/k, up/down)
- [x] Loading and error states render properly
- [x] SSH keys distinguished by origin (project vs instance)
- [x] Edit mode works with metadata editor component
- [x] Save functionality validates and persists metadata
- [x] Tests pass (27 tests covering all functionality)
- [x] Code passes linter with no issues
