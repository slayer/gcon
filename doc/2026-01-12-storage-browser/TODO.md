# Cloud Storage Bucket Browser

## Task Description
Implement a Google Cloud Storage bucket browser that allows users to:
- List buckets in a project
- Navigate into buckets to browse objects
- Browse folder hierarchy within buckets
- Pagination support with next/previous navigation

## Implementation Plan

### Phase 1: GCP Storage Client
- [x] Add `cloud.google.com/go/storage` dependency
- [x] Create `internal/gcp/storage.go` with:
  - `Bucket` struct
  - `StorageObject` struct
  - `StorageClient` with `ListBuckets()` and `ListObjects()` methods

### Phase 2: Buckets View
- [x] Create `internal/ui/views/buckets.go`
- [x] Implement list view following `instances.go` pattern
- [x] Key bindings: Enter (select), r (refresh), / (filter)

### Phase 3: Objects View
- [x] Create `internal/ui/views/objects.go`
- [x] Implement folder navigation with prefix stack
- [x] Pagination with n/p keys (page size: 100)
- [x] Key bindings: Enter, ESC, r, /, n, p

### Phase 4: App Integration
- [x] Add `ViewObjects` to ViewType enum
- [x] Add `bucketsView`, `objectsView`, `selectedBucket` fields
- [x] Handle `BucketSelectedMsg` and navigation
- [x] Update sidebar navigation handler

### Phase 5: Testing
- [x] Write storage client tests
- [x] Write BucketsView tests
- [x] Write ObjectsView tests
- [x] Run full test suite and linter

## Files Created
- `internal/gcp/storage.go`
- `internal/gcp/storage_test.go`
- `internal/ui/views/buckets.go`
- `internal/ui/views/buckets_test.go`
- `internal/ui/views/objects.go`
- `internal/ui/views/objects_test.go`

## Files Modified
- `go.mod`
- `internal/ui/app.go`
