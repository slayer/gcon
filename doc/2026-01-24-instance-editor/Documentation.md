# VM Instance Editor - Technical Documentation

## Overview

The VM Instance Editor feature allows users to edit instance properties (starting with labels) through a full-page editor view with state machine pattern and optimistic locking.

## Architecture

### State Machine

The editor follows a strict state machine pattern:

```
                  ┌─────────┐
                  │ Loading │
                  └────┬────┘
                       │ labelsLoadedMsg
                       ▼
                  ┌────────┐
          ┌──────▶│  Form  │◀──────┐
          │       └───┬────┘       │
          │           │ Ctrl+S     │ CancelMsg
          │           ▼            │
          │       ┌────────┐       │
          │       │  Diff  │───────┘
          │       └───┬────┘
          │           │ ConfirmMsg
          │           ▼
          │       ┌─────────┐
          │       │ Saving  │
          │       └───┬─────┘
          │           │
      retry           ▼
          │       ┌─────────┐
          └───────│  Error  │
                  └─────────┘
                       │ labelsSavedMsg
                       ▼
              InstanceEditCompleteMsg
```

### Component Hierarchy

```
App
└── InstanceEditorView
    ├── LabelEditor (labeledit.Editor)
    │   └── textinput.Model (key/value inputs)
    └── DiffViewer (diff.Viewer)
```

### Message Flow

```
[Instance Details]
       │
       │ ActionMenu → 'l' Edit Labels
       ▼
InstanceEditRequestMsg
       │
       ▼
[App] handleInstanceEditRequest
       │
       │ Creates InstanceEditorView
       ▼
[Instance Editor] Init() → loadLabels()
       │
       │ labelsLoadedMsg
       ▼
[LabelEditor] User edits
       │
       │ SaveRequestedMsg (Ctrl+S)
       ▼
[DiffViewer] Shows changes
       │
       │ ConfirmMsg (y)
       ▼
[Instance Editor] saveLabels()
       │
       │ labelsSavedMsg
       ▼
InstanceEditCompleteMsg
       │
       ▼
[App] handleInstanceEditComplete
       │
       │ Pop view stack, refresh details
       ▼
[Instance Details] (refreshed)
```

## Components

### GCP API (`internal/gcp/compute_labels.go`)

```go
// Retrieves labels with fingerprint for optimistic locking
func (c *ComputeClient) GetInstanceLabelsFingerprint(
    ctx context.Context,
    projectID, zone, instanceName string,
) (*InstanceLabelsFingerprint, error)

// Updates labels using fingerprint (returns error if stale)
func (c *ComputeClient) SetInstanceLabels(
    ctx context.Context,
    projectID, zone, instanceName string,
    labels map[string]string,
    fingerprint string,
) error
```

### LabelEditor (`internal/ui/components/labeledit/`)

Manages key-value label pairs with:
- List navigation (j/k, up/down)
- Add mode (a) with key/value inputs
- Edit mode (e/enter) with Tab to switch fields
- Delete with undo for existing labels
- GCP validation rules

### DiffViewer (`internal/ui/components/diff/`)

Shows before/after comparison:
- Green (+) for additions
- Red (-) for removals
- Gray for unchanged
- Yes/No confirmation buttons
- Keyboard navigation (←/→, Tab, Enter, y/n)

### InstanceEditorView (`internal/ui/views/instance_editor.go`)

Full-page editor that:
- Loads labels with fingerprint
- Hosts LabelEditor for editing
- Shows DiffViewer for confirmation
- Handles save with optimistic locking
- Emits navigation messages

## GCP Label Rules

Labels must follow these rules:
- **Keys**: Start with lowercase letter, contain only `[a-z0-9_-]`, max 63 chars
- **Values**: Contain only `[a-z0-9_-]`, max 63 chars, can be empty
- **Limits**: Max 64 labels per resource

## Error Handling

1. **Load Error**: Shows error with retry option (r key)
2. **Fingerprint Conflict**: "Labels were modified. Refresh and try again."
3. **Permission Denied**: Shows GCP error message
4. **Validation Error**: Shows inline error in LabelEditor

## Testing

```bash
# Run all new component tests
go test ./internal/gcp/compute_labels_test.go
go test ./internal/ui/components/diff/...
go test ./internal/ui/components/labeledit/...
go test ./internal/ui/views/... -run TestInstanceEditor
```

## Integration Points

### App Navigation
- `ViewInstanceEditor` added to ViewType enum
- Handlers in `app_navigation.go`:
  - `handleInstanceEditRequest`
  - `handleInstanceEditComplete`
  - `handleInstanceEditCancelled`

### Instance Details
- Action menu entry: `{Key: 'l', Label: "Edit Labels", Enabled: true}`
- Emits `InstanceEditRequestMsg` on selection

### Breadcrumbs
- Shows: `Project → Compute Engine → instance-name → Edit Labels`
