# GCS File Upload/Download Feature

## Task Description
Implement file upload and download functionality for the Cloud Storage browser:
- Download files from GCS to local current working directory
- Upload files from local filesystem to GCS
- Custom file picker with multi-select support
- Progress bar for transfers
- Conflict resolution prompts

## Implementation Plan

### Phase 1: Download Functionality
- [x] Add `DownloadObject()` method to StorageClient
- [x] Add progress bar component
- [x] Add 'd' key binding in ObjectsView
- [x] Implement recursive folder download
- [x] Show progress during download

### Phase 2: File Picker Component
- [x] Create `internal/ui/components/filepicker/filepicker.go`
- [x] Implement local filesystem browsing
- [x] Multi-select support with Space key
- [x] Modal overlay rendering

### Phase 3: Upload Functionality
- [x] Add `UploadObject()` method to StorageClient
- [x] Add 'u' key binding in ObjectsView
- [x] Open FilePickerView modal on 'u'
- [x] Upload with folder structure preservation
- [x] Show progress for each file

### Phase 4: Polish
- [ ] Conflict resolution dialog (overwrite/skip/rename)
- [ ] Cancel in-progress transfers
- [ ] Error handling and retry

## Files to Create
- [x] `internal/ui/components/filepicker/filepicker.go`
- [x] `internal/ui/components/filepicker/filepicker_test.go`
- [x] `internal/ui/components/progress/progress.go`
- [x] `internal/ui/components/progress/progress_test.go`

## Files to Modify
- [x] `internal/gcp/storage.go`
- [x] `internal/ui/views/objects.go`
