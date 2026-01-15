# Compute Instance Metadata & SSH Keys View

## Task ID: 2026-01-15

## Overview
Implement a new view for managing compute instance metadata and SSH keys. Users can view, add, edit, and delete metadata key-value pairs and SSH keys through a vim-like editor interface.

## Requirements

### Core Features
1. **Metadata Display**
   - Show all custom metadata key-value pairs for an instance
   - Display project-wide SSH keys (inherited from project metadata)
   - Display instance-specific SSH keys
   - Distinguish between project-level and instance-level metadata

2. **Metadata Editing**
   - In-place editor with vim-like interface
   - Support batch operations (add/delete multiple keys at once)
   - Validate key names and values before submission
   - Show loading state during API operations

3. **SSH Key Management**
   - Display project-wide SSH keys (read-only, show origin)
   - Display instance-specific SSH keys
   - Allow adding new SSH public keys
   - Allow deleting instance-specific SSH keys (not project-wide)
   - Validate SSH key format before submission

4. **Navigation**
   - Add "Metadata" item to sidebar navigation
   - Accessible when an instance is selected
   - Navigate back to previous view with 'esc'

### User Interface Design

#### Metadata View Structure
```
┌─────────────────────────────────────────────────┐
│ Instance Metadata - my-instance                 │
├─────────────────────────────────────────────────┤
│                                                 │
│ Custom Metadata                                 │
│   startup-script: #!/bin/bash...               │
│   environment:    production                    │
│   app-version:    1.2.3                        │
│                                                 │
│ SSH Keys (Project)                              │
│   john@example.com (from project metadata)     │
│     ssh-rsa AAAAB3NzaC1...                      │
│                                                 │
│ SSH Keys (Instance)                             │
│   jane@example.com                              │
│     ssh-rsa AAAAB3NzaC1...                      │
│                                                 │
├─────────────────────────────────────────────────┤
│ e: edit • a: add key • d: delete • esc: back    │
└─────────────────────────────────────────────────┘
```

#### Edit Mode Interface
- Show editor overlay with key-value pairs in editable format
- Format: `key=value` or `key: value` per line
- Support multi-line values (for SSH keys, startup scripts)
- Save with Ctrl+S or dedicated key binding
- Cancel with Esc

### Technical Approach

#### 1. GCP API Integration
**File:** `internal/gcp/compute.go`

Add methods to ComputeClient:
- `GetInstanceMetadata(ctx, projectID, zone, instanceName) (map[string]string, error)`
- `SetInstanceMetadata(ctx, projectID, zone, instanceName, metadata map[string]string, fingerprint string) error`
- `GetProjectMetadata(ctx, projectID) (map[string]string, error)`

Note: GCP metadata API requires a "fingerprint" for optimistic concurrency control.
Each update must include the current fingerprint to prevent race conditions.

#### 2. View Implementation
**File:** `internal/ui/views/instance_metadata.go`

Create `InstanceMetadataView` struct:
```go
type InstanceMetadataView struct {
    computeClient   *gcp.ComputeClient
    projectID       string
    zone            string
    instanceName    string
    metadata        map[string]string // Instance metadata
    projectMetadata map[string]string // Project metadata (read-only)
    fingerprint     string            // For optimistic locking
    viewport        viewport.Model
    editor          *metadataEditor   // Editor component
    loading         bool
    editMode        bool
    selectedKey     string
    err             error
}
```

View states:
- Loading: Fetching metadata from GCP
- Viewing: Display metadata with navigation
- Editing: In-place editor active
- Saving: Submitting changes to GCP

#### 3. Editor Component
**File:** `internal/ui/components/metadata_editor.go`

Create a text editor component for batch editing:
```go
type MetadataEditor struct {
    textarea  textarea.Model  // From bubbles/textarea
    content   string
    width     int
    height    int
}
```

Features:
- Parse key-value pairs from text
- Validate syntax and format
- Support multi-line values (quoted strings)
- Syntax highlighting for keys vs values

#### 4. SSH Key Handling

SSH keys in GCP metadata use special keys:
- Project-wide: `ssh-keys` key in project metadata
- Instance: `ssh-keys` key in instance metadata

Format: `username:ssh-rsa KEY_DATA user@host`

Parse and display separately:
- Extract username and key type
- Truncate long keys for display
- Show origin (project vs instance)

#### 5. Sidebar Integration
**File:** `internal/ui/components/sidebar/sidebar.go`

Add metadata view to sidebar:
- Show "Metadata" under Compute Engine section
- Only enable when an instance is selected
- Store selected instance in app state

#### 6. App Integration
**File:** `internal/ui/app.go`

Add to App struct:
- `ViewMetadata` to ViewType enum
- `metadataView *views.InstanceMetadataView`
- Handle `MetadataSelectedMsg` message

Navigation flow:
1. User selects "Metadata" from sidebar
2. App checks if instance is selected
3. Create metadata view with selected instance details
4. Navigate to metadata view

## Implementation Steps

### Phase 1: API & Data Layer
- [ ] Add metadata methods to `ComputeClient` in `internal/gcp/compute.go`
  - [ ] `GetInstanceMetadata`
  - [ ] `SetInstanceMetadata`
  - [ ] `GetProjectMetadata`
- [ ] Add tests for GCP API methods
- [ ] Handle fingerprint for optimistic locking
- [ ] Parse SSH keys from metadata

### Phase 2: View Implementation
- [ ] Create `internal/ui/views/instance_metadata.go`
- [ ] Implement view struct with loading/viewing/editing states
- [ ] Add viewport for scrollable content
- [ ] Render metadata key-value pairs
- [ ] Render SSH keys (project vs instance)
- [ ] Handle navigation (j/k, up/down)
- [ ] Add tests for view rendering

### Phase 3: Editor Component
- [ ] Create `internal/ui/components/metadata_editor.go`
- [ ] Integrate bubbles/textarea for text editing
- [ ] Implement key-value parser
- [ ] Add validation for metadata format
- [ ] Add validation for SSH key format
- [ ] Handle multi-line values
- [ ] Add tests for parser and validation

### Phase 4: UI Integration
- [ ] Add metadata view to sidebar navigation
- [ ] Update app.go to handle metadata view
- [ ] Add ViewMetadata to ViewType enum
- [ ] Wire up navigation messages
- [ ] Update view sizing logic
- [ ] Add key bindings for edit/add/delete

### Phase 5: Save & Update
- [ ] Implement save operation (send to GCP)
- [ ] Handle fingerprint updates
- [ ] Show loading state during save
- [ ] Handle errors (conflicts, validation, API)
- [ ] Show success/error messages
- [ ] Refresh metadata after save

### Phase 6: Testing & Documentation
- [ ] Add unit tests for all components
- [ ] Add integration tests for metadata operations
- [ ] Test error scenarios (conflicts, invalid keys)
- [ ] Update CLAUDE.md with metadata view documentation
- [ ] Update key bindings documentation
- [ ] Create Documentation.md with implementation details

## Key Bindings

### Metadata View (Viewing Mode)
- `e` - Enter edit mode
- `a` - Add new metadata key (opens editor with template)
- `d` - Delete selected key (with confirmation)
- `j/k` or `↓/↑` - Navigate through keys
- `r` - Refresh metadata from GCP
- `esc` - Go back to previous view

### Metadata View (Edit Mode)
- `Ctrl+S` - Save changes
- `Esc` - Cancel and return to viewing mode
- Standard text editing keys (handled by textarea)

## Error Handling

1. **Conflict Errors**: If fingerprint is stale, reload metadata and ask user to retry
2. **Validation Errors**: Show inline errors for invalid keys/values
3. **API Errors**: Display error message with retry option
4. **SSH Key Errors**: Validate format before attempting save

## Testing Strategy

1. **Unit Tests**
   - Metadata parser (key-value extraction)
   - SSH key parser and validator
   - View state transitions
   - Editor component

2. **Integration Tests**
   - Full metadata edit flow
   - Conflict resolution (fingerprint)
   - Navigation and view switching

3. **Manual Testing**
   - Test with real GCP instance
   - Try various metadata formats
   - Test SSH key addition/deletion
   - Verify project vs instance metadata display

## Technical Considerations

### Optimistic Locking with Fingerprint
Every metadata update requires the current fingerprint. If another client modifies metadata between our read and write, the API returns a conflict error. Handle this by:
1. Detect 412 Precondition Failed error
2. Reload metadata (new fingerprint)
3. Prompt user to retry or show diff of changes

### SSH Key Format
SSH keys in metadata are newline-separated entries:
```
user1:ssh-rsa AAAA... user1@host
user2:ssh-ed25519 AAAA... user2@host
```

Parse carefully to:
- Extract username prefix
- Identify key type (rsa, ed25519, ecdsa)
- Preserve entire key data
- Handle malformed entries gracefully

### Metadata Size Limits
GCP has limits:
- Max 256 KB total metadata size per instance
- Max 32 KB per key
- Max 256 keys

Validate before submission and show helpful error messages.

## Branch & Commit Strategy

**Branch:** `2026-01-15-compute-metadata-view`

**Commit Message Format:**
```
2026-01-15: <short description>

- Detail 1
- Detail 2
```

## Dependencies

- `github.com/charmbracelet/bubbles/textarea` - Text editor component
- `google.golang.org/api/compute/v1` - GCP Compute API (already included)

## Out of Scope

- Editing project-level metadata (requires project-level permissions)
- Bulk operations across multiple instances
- Metadata templates or presets
- Metadata history or versioning
- OS Login SSH key integration (different API)

## Future Enhancements

- Syntax highlighting in editor
- Metadata search/filter
- Copy metadata between instances
- Import/export metadata as JSON/YAML
- Metadata templates for common configurations
