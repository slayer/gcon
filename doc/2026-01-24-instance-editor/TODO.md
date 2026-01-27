# VM Instance Editor - Implementation Tracking

**Date:** 2026-01-24
**Branch:** `2026-01-18-resource-editing`

## Status: ✅ Complete (MVP - Labels Editing)

## Tasks Completed

- [x] **GCP API Methods** (`internal/gcp/compute_labels.go`)
  - `GetInstanceLabelsFingerprint()` - Retrieves labels and fingerprint for optimistic locking
  - `SetInstanceLabels()` - Updates labels using fingerprint

- [x] **DiffViewer Component** (`internal/ui/components/diff/`)
  - Shows before/after comparison for changes
  - Yes/No confirmation with keyboard navigation
  - Color-coded additions (green) and removals (red)

- [x] **LabelEditor Component** (`internal/ui/components/labeledit/`)
  - Key-value pair editor with cursor navigation
  - Add (a), Edit (e/enter), Delete (x) operations
  - GCP label validation (lowercase, numbers, hyphens, underscores)
  - Tracks dirty state, new labels, and deletions
  - Tab to switch between key/value inputs

- [x] **InstanceEditorView** (`internal/ui/views/instance_editor.go`)
  - State machine: Loading → Form → Diff → Saving → Done/Error
  - Fingerprint-based optimistic locking
  - Error handling with retry capability
  - Emits messages for navigation integration

- [x] **App Integration**
  - Added `ViewInstanceEditor` to view types
  - Added `instanceEditorView` field to App
  - Added handlers: `handleInstanceEditRequest`, `handleInstanceEditComplete`, `handleInstanceEditCancelled`
  - Updated `getCurrentViewModel`, `updateViewSizes`, `clearAllViews`
  - Updated breadcrumbs and footer

- [x] **Instance Details Integration**
  - Added "Edit Labels" (l) to action menu
  - Emits `InstanceEditRequestMsg` when selected

## Files Changed

### New Files
```
internal/gcp/compute_labels.go
internal/gcp/compute_labels_test.go
internal/ui/components/diff/diff.go
internal/ui/components/diff/diff_test.go
internal/ui/components/labeledit/labeledit.go
internal/ui/components/labeledit/labeledit_test.go
internal/ui/views/instance_editor.go
internal/ui/views/instance_editor_test.go
```

### Modified Files
```
internal/ui/app.go - Added ViewInstanceEditor, instanceEditorView, message handlers
internal/ui/app_navigation.go - Added handleInstanceEdit* functions, clearAllViews update
internal/ui/app_render.go - Added ViewInstanceEditor to rendering and breadcrumbs
internal/ui/app_footer.go - Added ViewInstanceEditor to navigation hint
internal/ui/views/instance_details.go - Added Edit Labels action
```

## User Flow

1. Navigate to Instance Details view
2. Press `.` to open action menu
3. Press `l` or select "Edit Labels"
4. Editor loads with current labels
5. Use `j/k` to navigate, `a` to add, `e` to edit, `x` to delete
6. Press `Ctrl+S` to preview changes
7. DiffViewer shows changes with confirmation
8. Press `y` or Enter (with Yes focused) to save
9. Returns to Instance Details (refreshed)

## Key Bindings

### Label Editor
| Key | Action |
|-----|--------|
| `↑/k` | Move up |
| `↓/j` | Move down |
| `a` | Add new label |
| `e`/`Enter` | Edit selected label |
| `x`/`Delete` | Delete selected label |
| `Ctrl+S` | Preview changes |
| `Esc` | Cancel |

### Diff Viewer
| Key | Action |
|-----|--------|
| `←/h` | Select Yes |
| `→/l` | Select No |
| `Tab` | Toggle selection |
| `Enter` | Confirm selection |
| `y` | Confirm changes |
| `n`/`Esc` | Cancel |

## Future Enhancements (Phase 2+)

- [ ] Tags editing
- [ ] Description editing
- [ ] Deletion protection toggle
- [ ] Machine type editing (requires stop/start workflow)
