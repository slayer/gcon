# Project-wide Metadata View Implementation

## Overview

This document describes the implementation of the project-wide metadata (common instance metadata) view and editing feature. This metadata applies to ALL instances in a GCP project and is managed separately from per-instance metadata.

## Summary of Changes

### API Layer (`internal/gcp/compute.go`)

1. **Updated `GetProjectMetadata`**:
   - Changed return type from `map[string]string` to `*InstanceMetadata`
   - Now returns both metadata items and fingerprint for optimistic locking
   - This is project-level "common instance metadata" that applies to all instances

2. **Added `SetProjectMetadata`**:
   - New method to update project-wide metadata
   - Uses `Projects.SetCommonInstanceMetadata` API endpoint
   - Requires fingerprint for optimistic locking to prevent concurrent update conflicts
   - Converts metadata map to GCP API format with `compute.Metadata` struct

### View Layer (`internal/ui/views/project_metadata.go`)

3. **Created `ProjectMetadataView`**:
   - Similar structure to `InstanceMetadataView` but for project-level metadata
   - Key differences:
     - Title: "Project Metadata - {projectID}"
     - Shows warning before saving: "This will affect ALL instances in {projectID}"
     - No separate "Project SSH Keys" section (this IS the project metadata)
     - SSH keys displayed prominently as they're the most common use case

4. **View States**:
   - Loading: Fetching metadata from GCP
   - Viewing: Display metadata with scroll navigation
   - Editing: In-place metadata editor
   - Warning: Confirmation dialog before save
   - Saving: Submitting changes to GCP

5. **UI Features**:
   - Displays SSH keys (project-wide) prominently
   - Shows custom metadata (non-SSH) separately
   - Edit mode uses existing `MetadataEditor` component
   - Warning dialog requires explicit confirmation (Ctrl+S) before saving
   - Success message after save with reminder about instance impact

### Sidebar Integration

6. **Updated Sidebar Menu** (`internal/ui/components/sidebar/menu.go`):
   - Added `ViewProjectMetadata` to `ViewType` enum
   - Added "Project metadata" menu item under Compute Engine
   - Uses same metadata icon as instance metadata
   - Now has 4 items under Compute Engine:
     1. VM instances
     2. Disks
     3. Metadata (instance-specific)
     4. Project metadata (project-wide)

7. **Updated Sidebar Tests**:
   - Updated test assertions to expect 4 children under Compute Engine (was 3)
   - All tests pass

### App Integration (`internal/ui/app.go`)

8. **Added `ViewProjectMetadata`**:
   - Added to main `ViewType` enum
   - Added `projectMetadataView` field to App struct
   - Wire up navigation from sidebar
   - Handle back navigation (esc key)
   - Update view sizing and context propagation
   - Update breadcrumb header to show "Project Metadata"

9. **Navigation Flow**:
   ```
   Sidebar -> Project metadata -> ProjectMetadataView.Init() -> Load metadata -> Display
   ```

10. **View Cleanup**:
    - Project metadata view is cleaned up when navigating back
    - Properly disposed when switching projects

### Updated Instance Metadata View

11. **Fixed `InstanceMetadataView`** (`internal/ui/views/instance_metadata.go`):
    - Updated to use new `GetProjectMetadata` signature
    - Changed `projectMetadata` type from `map[string]string` to `*InstanceMetadata`
    - Updated `parseProjectSSHKeys` to access `projectMetadata.Items`

12. **Updated Tests**:
    - Fixed all test assertions in `instance_metadata_test.go`
    - Changed mock project metadata from `map[string]string` to `*InstanceMetadata`
    - All tests pass

## Technical Details

### Fingerprint-based Optimistic Locking

Project metadata uses fingerprint for optimistic concurrency control:

1. When loading metadata: Save the fingerprint from `project.CommonInstanceMetadata.Fingerprint`
2. When saving metadata: Include the fingerprint in the update request
3. If another client modified metadata between our read and write:
   - GCP API returns 412 Precondition Failed
   - We reload metadata (new fingerprint) and prompt user to retry

### SSH Keys in Project Metadata

SSH keys are stored in the `ssh-keys` metadata key:
- Format: `username:ssh-rsa KEY user@host\nusername2:ssh-ed25519 KEY2 user2@host2`
- Each key is on a new line
- All instances in the project inherit these keys
- Instances can override with instance-specific SSH keys

### Metadata Size Limits

GCP enforces these limits on project metadata:
- Max 256 KB total metadata size
- Max 32 KB per key
- Max 256 keys per project

The metadata editor validates these limits before saving.

## Files Modified

### New Files
- `internal/ui/views/project_metadata.go` - Project metadata view implementation
- `doc/2026-01-16-project-metadata/TODO.md` - Task tracking
- `doc/2026-01-16-project-metadata/Documentation.md` - This file

### Modified Files
- `internal/gcp/compute.go` - Updated GetProjectMetadata, added SetProjectMetadata
- `internal/ui/views/instance_metadata.go` - Updated to use new GetProjectMetadata signature
- `internal/ui/views/instance_metadata_test.go` - Fixed test mocks
- `internal/ui/components/sidebar/menu.go` - Added project metadata menu item
- `internal/ui/components/sidebar/sidebar_test.go` - Updated test assertions
- `internal/ui/app.go` - Integrated project metadata view

## Testing

### Unit Tests
- All existing tests pass
- Updated instance metadata view tests to use new API signature
- Updated sidebar tests to expect 4 children under Compute Engine

### Integration Testing
Run manually:
```bash
# Build and run
make build
./bin/gcon

# Navigate to project
# Select "Project metadata" from sidebar
# Edit metadata
# Save (confirm warning)
# Verify save success message
```

### Linter
```bash
make lint
# 0 issues
```

## Key Bindings

### Project Metadata View - Viewing Mode
- `e` - Enter edit mode
- `r` - Refresh metadata from GCP
- `esc` - Go back to previous view
- `↑/↓` or `j/k` - Scroll content

### Project Metadata View - Edit Mode
- `Ctrl+S` - Show save confirmation warning
- `Esc` - Cancel edit mode

### Project Metadata View - Warning Mode
- `Ctrl+S` - Confirm and save changes
- `Esc` - Cancel save, return to edit mode

## UI/UX Considerations

### Warning Before Save

The view shows a prominent warning before saving:
```
⚠️  WARNING: This will affect ALL instances in project-name

Are you sure you want to save?

ctrl+s: confirm save • esc: cancel
```

This is critical because project metadata affects all instances in the project.

### Info Text

In view mode:
```
This metadata applies to ALL instances in the project.
Instance-specific metadata can override these values.
```

This helps users understand the scope and inheritance model.

### Success Message

After successful save:
```
✓ Project metadata saved successfully!
This affects all instances in the project.
```

Reinforces the project-wide impact of the change.

## Future Enhancements

Potential improvements (out of scope for this task):
- Show count of instances affected by metadata change
- Show which instances inherit this metadata
- Diff view showing changes before save
- Metadata validation with suggestions
- Import/export metadata as JSON/YAML
- Metadata templates for common configurations

## Related Documentation

- Instance Metadata View: `doc/2026-01-15-compute-metadata-view/Documentation.md`
- GCP Metadata API: https://cloud.google.com/compute/docs/metadata/overview
- Common Instance Metadata: https://cloud.google.com/compute/docs/metadata/setting-custom-metadata#project-wide-metadata
