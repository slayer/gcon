# GCS Delete Files Feature - Documentation

## Summary

Added the ability to delete single files or entire folders from GCS buckets with mandatory user confirmation before deletion.

## Changes Made

### New Files

1. **`internal/ui/components/confirm/confirm.go`** - Reusable confirmation dialog component
   - Modal dialog with Yes/No buttons
   - Keyboard navigation: Left/Right/Tab to switch, Enter to confirm, y/n shortcuts
   - GCP-styled border (red for destructive actions)
   - Supports title, message, and optional detail lines

2. **`internal/ui/components/confirm/confirm_test.go`** - Tests for confirmation dialog

### Modified Files

1. **`internal/gcp/storage.go`** - Added delete methods:
   - `DeleteObject(ctx, bucketName, objectName)` - Delete single object
   - `DeleteObjects(ctx, bucketName, objectNames, progress)` - Delete multiple objects with progress

2. **`internal/ui/views/objects.go`** - Added delete functionality:
   - Key binding: `D` (shift+d) for delete
   - Delete state fields: `pendingDelete`, `pendingDeleteFiles`, `showDeleteConfirm`, etc.
   - Message types: `deleteRequestMsg`, `deleteFilesResolvedMsg`, `deleteStartMsg`, `deleteProgressMsg`, `deleteCompleteMsg`
   - Helper methods: `prepareDelete`, `resolveDeleteFiles`, `createDeleteConfirmDialog`, `startDelete`, etc.
   - Overlay methods: `overlayDeleteConfirm`, `overlayDeleteProgress`

3. **`internal/ui/views/objects_test.go`** - Added tests for delete functionality

4. **`CLAUDE.md`** - Added Objects View key bindings section

## Technical Details

### Message Flow

```
User presses 'D' on selected object
        │
        ▼
deleteRequestMsg{object}
        │
        ├── Single file: Return deleteFilesResolvedMsg directly
        │
        └── Folder: Call ListAllObjects to get all files
                    │
                    ▼
             deleteFilesResolvedMsg{files}
                    │
                    ▼
        Show ConfirmDialog with file count/names
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
  confirm.ConfirmMsg       confirm.CancelMsg
        │                       │
        ▼                       ▼
deleteStartMsg{files}    Clear state, return
        │
        ▼
Start goroutine, poll deleteChan
        │
        ├── deleteProgressMsg (per file)
        │           │
        │           ▼
        │    Update progress display
        │           │
        │           ▼
        │    waitForDeleteProgress()
        │
        └── deleteCompleteMsg (on done/error)
                    │
                    ▼
            Clear state, refresh list (or show error)
```

### Key Binding

- `D` (shift+d) for delete
- Distinct from `d` (download) to prevent accidental deletion
- Uppercase convention for destructive actions (like `R` for reset)

### Confirmation Dialog

- Defaults to "No" button focused for safety
- Shows file count and preview (first 5 files + "... and X more")
- Red border to indicate destructive action
- Quick keys: `y` for yes, `n`/`Esc` for cancel

### Error Handling

- Fail-fast on first error during multi-file deletion
- Reports partial success: "deleted X files, failed on 'filename': error"
- Refreshes list on success, shows error on failure

## Testing

```bash
# Run all tests
make test

# Run specific tests
go test ./internal/ui/components/confirm/... -v
go test ./internal/ui/views/... -v -run TestObjectsViewDelete
```

### Manual Testing Checklist

- [ ] Single file deletion with confirmation
- [ ] Cancel single file deletion (press 'n' or Esc)
- [ ] Folder deletion with file count display
- [ ] Cancel folder deletion
- [ ] Delete key ignored during loading state
- [ ] Delete key ignored during download/upload/delete
- [ ] Verify list refreshes after successful deletion
- [ ] Test error display on delete failure

## Key Bindings Reference

| Key | Action |
|-----|--------|
| `D` | Delete selected file/folder |
| `y` | Confirm deletion (in dialog) |
| `n` | Cancel deletion (in dialog) |
| `Esc` | Cancel deletion (in dialog) |
| `←/→` | Switch between Yes/No buttons |
| `Tab` | Switch between Yes/No buttons |
| `Enter` | Confirm focused button |
