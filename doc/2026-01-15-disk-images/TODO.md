# Disk Images Feature

## Task Description

Implement disk images management feature for gcon, allowing users to:
1. List all disk images in a project
2. View detailed information about a specific disk image
3. Delete disk images with confirmation

## Implementation Plan

### 1. GCP Client Layer (`internal/gcp/`)

**File: `internal/gcp/images.go`** (new)
- `ListImages(ctx, projectID, zone)` - List all disk images
- `GetImage(ctx, projectID, imageName)` - Get image details
- `DeleteImage(ctx, projectID, imageName)` - Delete an image

Key considerations:
- Images can be at project level or global
- Need to handle both custom and public images appropriately
- Delete operations are asynchronous (return operation status)

### 2. UI Views (`internal/ui/views/`)

**File: `internal/ui/views/images.go`** (new)
- Images list view with filterable table
- Display: name, family, status, size, creation date
- Key bindings:
  - `Enter` - View image details
  - `D` - Delete image (with confirmation)
  - `r` - Refresh list
  - `Esc` - Go back

**File: `internal/ui/views/image_details.go`** (new)
- Detailed view showing:
  - Basic info (name, family, description)
  - Size and disk specs
  - Source disk/snapshot
  - Creation timestamp
  - Labels
  - Licenses
  - Status
- Key bindings:
  - `D` - Delete image
  - `Esc` - Go back to list

### 3. Navigation Integration

**File: `internal/ui/app.go`**
- Add `ImagesView` and `ImageDetailsView` to view enum
- Add navigation from projects view → images view
- Wire up message passing for image selection

### 4. Messages (`internal/ui/messages.go`)

Add new message types:
- `ImageSelectedMsg` - Navigate to image details
- `ImageDeletedMsg` - Refresh list after deletion
- `ImagesErrorMsg` - Handle errors

### 5. Sidebar Integration

Add "Images" entry to sidebar navigation (similar to Instances, Disks)

## Technical Considerations

1. **API Permissions**: Requires `compute.images.list`, `compute.images.get`, `compute.images.delete`
2. **Pagination**: Images list may need pagination for projects with many images
3. **Image Types**: Handle different image types (custom, public, deprecated)
4. **Delete Confirmation**: Require explicit confirmation to prevent accidental deletion
5. **Error Handling**: Images may fail to delete if in use by instances

## Testing Strategy

1. **Unit Tests**:
   - GCP client methods (mocked API calls)
   - View rendering and state management
   - Message handling

2. **Integration Tests**:
   - Full navigation flow
   - Delete confirmation flow

## Success Criteria

- [ ] Users can list all disk images in a project
- [ ] Users can view detailed information about any image
- [ ] Users can delete images with confirmation dialog
- [ ] Proper error handling for API failures
- [ ] Tests pass with >80% coverage
- [ ] Linter passes with no warnings
