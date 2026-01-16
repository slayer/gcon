# Compute Engine Instance Metadata View

## Overview

This directory contains documentation for the Compute Engine instance metadata view feature implemented on 2026-01-15. The feature allows users to view and edit instance metadata through a terminal UI.

## Documentation Index

### Planning Documents

- **[TODO.md](TODO.md)** - Master task list for the entire feature
- **[TODO_api_layer.md](TODO_api_layer.md)** - GCP API integration tasks (Phase 1) ✓ Complete
- **[TODO_editor.md](TODO_editor.md)** - Metadata editor component tasks (Phase 2) ✓ Complete
- **[TODO_view_component.md](TODO_view_component.md)** - Main view component tasks (Phase 3) ✓ Complete
- **[TODO_integration.md](TODO_integration.md)** - App and sidebar integration tasks (Phase 4-5) ✓ Complete

### Completion Reports

- **[INTEGRATION_SUMMARY.md](INTEGRATION_SUMMARY.md)** - Technical summary of integration changes
- **[COMPLETION_REPORT.md](COMPLETION_REPORT.md)** - Final completion report with test results
- **[ARCHITECTURE_DIAGRAM.md](ARCHITECTURE_DIAGRAM.md)** - Component diagrams and data flow

## Feature Summary

The metadata view provides:

1. **View Instance Metadata**: Display instance-specific and project-wide metadata
2. **Edit Metadata**: Text-based editor for custom metadata
3. **Save to GCP**: Updates metadata with optimistic locking (fingerprint)
4. **SSH Key Display**: Read-only display of project and instance SSH keys
5. **Error Handling**: Robust error handling for conflicts and validation errors

## Implementation Phases

### Phase 1: GCP API Layer ✓
- Implemented `GetInstanceMetadata()` in ComputeClient
- Implemented `SetInstanceMetadata()` with fingerprint support
- Implemented `GetProjectMetadata()` for project-wide metadata
- Created SSH key parsing utilities
- Added comprehensive tests

### Phase 2: Metadata Editor Component ✓
- Created MetadataEditor component based on textarea
- Implemented metadata parsing (key=value format)
- Added validation rules (key format, value length, reserved keys)
- Created serialization for display
- Added comprehensive tests

### Phase 3: View Component ✓
- Created InstanceMetadataView with viewport and editor
- Implemented view mode (read-only display)
- Implemented edit mode (interactive editing)
- Added save operations with error handling
- Integrated spinner for loading/saving states
- Added comprehensive tests

### Phase 4-5: Integration ✓
- Added ViewMetadata to sidebar menu
- Integrated metadata view into app navigation
- Implemented proper state management
- Added context propagation
- Updated all tests

## Files Created

### Source Files
```
internal/gcp/
  ├─ metadata.go              # SSH key parsing utilities
  └─ metadata_test.go         # Tests for metadata utilities

internal/ui/components/
  ├─ metadata_editor.go       # Metadata editor component
  └─ metadata_editor_test.go  # Tests for editor component

internal/ui/views/
  ├─ instance_metadata.go     # Main metadata view
  └─ instance_metadata_test.go # Tests for view
```

### Modified Files
```
internal/gcp/
  └─ compute.go               # Added GetInstanceMetadata, SetInstanceMetadata, GetProjectMetadata

internal/ui/
  └─ app.go                   # Added metadata view integration

internal/ui/components/sidebar/
  ├─ menu.go                  # Added ViewMetadata and menu item
  └─ sidebar_test.go          # Updated tests for 3 compute children
```

## Usage

### Navigation
1. Select a project
2. Select an instance (stores in app state)
3. Press Tab to focus sidebar
4. Navigate to "Compute Engine" → "Metadata"

### Viewing Metadata
- Scroll through metadata with ↑/↓ or j/k
- View custom metadata, project SSH keys, and instance SSH keys
- Press 'r' to refresh metadata

### Editing Metadata
- Press 'e' to enter edit mode
- Edit custom metadata in key=value format
- Press Ctrl+S to save changes
- Press Esc to cancel without saving

### Key Bindings
- `↑/↓` or `j/k`: Scroll
- `e`: Enter edit mode
- `r`: Refresh metadata
- `Ctrl+S`: Save changes (edit mode)
- `Esc`: Cancel edit / Go back

## Architecture

The feature follows a layered architecture:

```
App (Navigation & State)
  ↓
InstanceMetadataView (View Logic)
  ↓
├─ MetadataEditor (Edit Component)
├─ Viewport (Display Component)
└─ ComputeClient (GCP API)
```

See [ARCHITECTURE_DIAGRAM.md](ARCHITECTURE_DIAGRAM.md) for detailed diagrams.

## Testing

All components have comprehensive test coverage:

- **GCP API Layer**: Tests for metadata retrieval, updates, and parsing
- **Editor Component**: Tests for parsing, validation, and serialization
- **View Component**: Tests for loading, editing, saving, and error handling
- **Integration**: Updated sidebar tests for menu structure

Run tests:
```bash
make test
```

## Design Decisions

### 1. Instance Selection Required
The metadata view requires an instance to be selected. Enforced in `handleSidebarNavigation()`.

**Rationale**: Metadata is instance-specific, view would be meaningless without selection.

### 2. Compute Client Reuse
Metadata view receives compute client from instances view.

**Rationale**: Avoids re-initializing GCP client, improves performance.

### 3. Text-Based Editor
Metadata editing uses simple text format (key=value).

**Rationale**:
- Matches gcloud CLI format
- Familiar to developers
- Simple to implement and test

### 4. Read-Only SSH Keys
SSH keys are displayed but not editable in the metadata view.

**Rationale**:
- Complex format requires special handling
- Project keys managed at project level
- Reduces risk of breaking SSH access

### 5. Optimistic Locking
Uses fingerprint for conflict detection during saves.

**Rationale**: Prevents concurrent update issues, matches GCP API best practices.

## Future Enhancements

Potential improvements for future iterations:

1. **SSH Key Management**: Dedicated UI for adding/removing SSH keys
2. **Metadata Search**: Filter/search capabilities for large metadata sets
3. **Bulk Operations**: Apply metadata to multiple instances
4. **Templates**: Save and reuse common metadata configurations
5. **Direct Navigation**: Jump to metadata from instance details view

## References

- [GCP Instance Metadata Documentation](https://cloud.google.com/compute/docs/metadata/overview)
- [GCP API: instances.setMetadata](https://cloud.google.com/compute/docs/reference/rest/v1/instances/setMetadata)
- [Bubble Tea Framework](https://github.com/charmbracelet/bubbletea)

## Status

✅ **Complete and Merged**

All phases complete, all tests passing, ready for production use.
