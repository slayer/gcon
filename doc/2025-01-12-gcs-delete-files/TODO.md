# GCS Delete Files Feature

## Task Description

Add ability to delete single files or entire folders from GCS buckets with mandatory user confirmation before deletion.

## Requirements

- Delete single files with confirmation
- Delete folders (recursive delete of all objects with prefix)
- Key binding: `D` (shift+d)
- Show progress during multi-file deletion
- Handle partial failures gracefully

## Implementation Plan

- [x] Create feature branch
- [x] Create confirmation dialog component (`internal/ui/components/confirm/`)
- [x] Add `DeleteObject` and `DeleteObjects` methods to storage client
- [x] Update ObjectsView with delete state, key binding, message handlers
- [x] Add overlay rendering for confirmation and progress
- [x] Write unit tests
- [x] Update CLAUDE.md key bindings
- [x] Run linter and tests
- [x] Create Documentation.md

## Files to Create/Modify

| File | Status | Description |
|------|--------|-------------|
| `internal/ui/components/confirm/confirm.go` | DONE | New confirmation dialog component |
| `internal/ui/components/confirm/confirm_test.go` | DONE | Component tests |
| `internal/gcp/storage.go` | DONE | Add delete methods |
| `internal/ui/views/objects.go` | DONE | Delete functionality |
| `internal/ui/views/objects_test.go` | DONE | Delete tests |
| `CLAUDE.md` | DONE | Update key bindings |
