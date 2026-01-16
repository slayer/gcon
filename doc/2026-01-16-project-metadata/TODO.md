# Project-wide Metadata View & Editing

## Task ID: 2026-01-16

## Overview
Implement project-wide metadata (common instance metadata) view and editing. This metadata applies to ALL instances in a project and is different from per-instance metadata.

## Requirements

### Core Features
1. **Display project metadata** that applies to all instances
2. **Edit project metadata** with save functionality
3. **Show SSH keys** in project metadata (most common use case)
4. **Warning before saving** - emphasize this affects all instances
5. **Fingerprint-based optimistic locking** to prevent conflicts

### API Changes

#### File: `internal/gcp/compute.go`

1. **Update GetProjectMetadata** to return fingerprint:
   - Change signature: `GetProjectMetadata(ctx, projectID) (*InstanceMetadata, error)`
   - Currently returns `map[string]string` but needs fingerprint too
   - Return `InstanceMetadata` struct (reuse from instance metadata)

2. **Add SetProjectMetadata**:
   - `SetProjectMetadata(ctx, projectID, metadata, fingerprint) error`
   - Use `c.service.Projects.SetCommonInstanceMetadata(projectID, metadata).Context(ctx).Do()`
   - Convert metadata map to `compute.Metadata` with Items array
   - Include fingerprint for optimistic locking
   - Handle errors with `WrapActionError`

#### File: `internal/gcp/compute_test.go`

3. **Add tests** for SetProjectMetadata

### View Implementation

#### File: `internal/ui/views/project_metadata.go` (NEW)

Create `ProjectMetadataView` similar to `InstanceMetadataView`:

**Key differences from instance metadata:**
- Title: "Project Metadata - {projectID}"
- Help text explains this applies to ALL instances
- No separate "Project SSH Keys" section (this IS the project metadata)
- Show warning before saving: "This will affect all instances in {projectID}"
- Show instance count affected (optional)

**Struct:**
```go
type ProjectMetadataView struct {
    computeClient    *gcp.ComputeClient
    projectID        string
    ctx              *context.ProgramContext

    // Metadata state
    metadata         *gcp.InstanceMetadata  // Project metadata with fingerprint
    fingerprint      string

    // UI state
    viewport         viewport.Model
    editor           components.MetadataEditor
    spinner          spinner.Model
    loading          bool
    saving           bool
    editMode         bool
    err              error
    saveSuccess      bool
    width            int
    height           int
    keys             projectMetadataKeyMap
    ready            bool
}
```

**View states:**
1. Loading: Fetching metadata from GCP
2. Viewing: Display metadata with navigation
3. Editing: In-place editor active
4. Saving: Submitting changes to GCP

### Sidebar Integration

#### File: `internal/ui/components/sidebar/menu.go`

1. Add `ViewProjectMetadata` constant to `ViewType` enum
2. Add menu item under Compute Engine:
   ```go
   {ID: "project-metadata", Label: "Project metadata", Icon: IconMetadata, Type: MenuItemLeaf, ViewType: ViewProjectMetadata}
   ```

### App Integration

#### File: `internal/ui/app.go`

1. Add `ViewProjectMetadata` to `ViewType` enum
2. Add `projectMetadataView *views.ProjectMetadataView` to App struct
3. Handle navigation in `handleSidebarNavigation`:
   ```go
   case sidebar.ViewProjectMetadata:
       if a.currentView != ViewProjectMetadata {
           a.currentView = ViewProjectMetadata
           // Create view with compute client from instances view
           if a.instancesView != nil {
               a.projectMetadataView = views.NewProjectMetadataView(
                   a.selectedProject.ID,
                   a.instancesView.GetComputeClient(),
               )
               a.updateViewSizes()
               cmd = a.projectMetadataView.Init()
           }
       }
   ```
4. Handle back navigation (esc key)
5. Update `renderCurrentView` for `ViewProjectMetadata`
6. Update `updateSidebarActiveView` for `ViewProjectMetadata`

## Implementation Steps

### Phase 1: API Layer ✅
- [x] Update `GetProjectMetadata` to return `*InstanceMetadata` with fingerprint
- [x] Add `SetProjectMetadata` method
- [x] Add tests for `SetProjectMetadata`
- [x] Run tests to verify API changes

### Phase 2: View Component ✅
- [x] Create `internal/ui/views/project_metadata.go`
- [x] Implement `ProjectMetadataView` struct
- [x] Implement Init, Update, View, SetContext methods
- [x] Add loading/saving states with spinner
- [x] Add viewport for scrollable content
- [x] Add metadata editor integration
- [x] Add save confirmation with warning
- [x] Create tests in `project_metadata_test.go`

### Phase 3: Sidebar & Navigation ✅
- [x] Add `ViewProjectMetadata` to sidebar menu types
- [x] Add "Project metadata" menu item
- [x] Update sidebar tests

### Phase 4: App Integration ✅
- [x] Add `ViewProjectMetadata` to app ViewType enum
- [x] Add projectMetadataView to App struct
- [x] Wire up navigation in handleSidebarNavigation
- [x] Handle back navigation
- [x] Update renderCurrentView
- [x] Update updateSidebarActiveView
- [x] Update view sizing logic

### Phase 5: Testing & Validation ✅
- [x] Run all tests
- [x] Test navigation flow
- [x] Test edit and save flow
- [x] Test error handling (conflicts, validation)
- [ ] Test with real GCP project (requires manual testing)
- [x] Run linter

## UI/UX Considerations

### Warning Message
Before saving, show prominent warning:
```
⚠️  WARNING: This will affect ALL instances in project-name

Are you sure you want to save? (Ctrl+S to confirm, Esc to cancel)
```

### Help Text
In view mode:
```
This metadata applies to ALL instances in the project.
Instance-specific metadata can override these values.

↑/↓: scroll • e: edit • r: refresh • esc: back
```

### Save Confirmation
After save:
```
✓ Project metadata saved successfully!
This affects all XX instances in the project.
```

## Key Bindings

### View Mode
- `e` - Enter edit mode
- `r` - Refresh metadata from GCP
- `esc` - Go back to previous view
- `↑/↓` or `j/k` - Scroll content

### Edit Mode
- `Ctrl+S` - Save changes (with warning)
- `Esc` - Cancel and return to viewing mode

## Error Handling

1. **Conflict (412)**: Fingerprint stale, reload and prompt retry
2. **Validation**: Inline errors for invalid keys/values
3. **API errors**: Display with retry option
4. **SSH key errors**: Validate format before save

## Testing Strategy

1. **Unit Tests**
   - View rendering
   - State transitions
   - Editor integration
   - API method tests

2. **Integration Tests**
   - Full edit and save flow
   - Conflict resolution
   - Navigation

3. **Manual Testing**
   - Test with real GCP project
   - Verify SSH keys display correctly
   - Test save with multiple instances

## Technical Notes

### Fingerprint Handling
- Project metadata has its own fingerprint at `project.CommonInstanceMetadata.Fingerprint`
- Must include in every update for optimistic locking
- If conflict, reload and prompt user

### SSH Keys
- SSH keys in project metadata apply to all instances
- Format: `username:ssh-rsa KEY user@host\n...`
- Instances inherit these keys unless overridden

### Metadata Limits
- Same as instance metadata:
  - Max 256 KB total
  - Max 32 KB per key
  - Max 256 keys

## Branch & Commit Strategy

**Branch:** `2026-01-16-project-metadata`

**Commit Format:**
```
2026-01-16: <short description>
```

## Out of Scope
- Showing which instances inherit this metadata
- Bulk editing across multiple projects
- Metadata templates
- History/versioning

## Dependencies
- Reuse existing `components.MetadataEditor`
- Reuse existing `gcp.InstanceMetadata` struct
- Use existing patterns from `InstanceMetadataView`
