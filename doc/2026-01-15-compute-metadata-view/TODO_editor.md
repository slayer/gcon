# Phase 3: Editor Component

## Objective
Create a text editor component for batch editing metadata with vim-like interface.

## Tasks

- [x] Create `MetadataEditor` component
  - Wrap bubbles/textarea for text editing
  - Store original metadata for comparison
  - Track dirty state

- [x] Implement key-value parser
  - Parse text into metadata map
  - Support formats: `key=value` or `key: value`
  - Handle multi-line values (quoted or indented)
  - Detect SSH key entries

- [x] Implement metadata serializer
  - Convert metadata map to editable text
  - Format consistently
  - Handle special characters in values
  - Pretty-print SSH keys

- [x] Add validation
  - Validate key names (allowed characters)
  - Validate value sizes (GCP limits)
  - Validate SSH key format
  - Check total metadata size (256 KB limit)

- [ ] Implement editor UI
  - Show editor overlay or full-screen
  - Display validation errors inline
  - Show character/size limits
  - Show save/cancel instructions

- [ ] Add save confirmation
  - Show diff of changes
  - Confirm before submitting
  - Handle cancel

- [x] Create editor tests
  - Test parsing various formats
  - Test validation rules
  - Test serialization
  - Test SSH key handling

## Files to Create/Modify

- `internal/ui/components/metadata_editor.go` (new)
- `internal/ui/components/metadata_editor_test.go` (new)

## Acceptance Criteria

- Editor parses and formats metadata correctly
- Validation catches invalid inputs
- SSH keys handled properly
- UI is intuitive and shows helpful errors
- Tests cover edge cases
