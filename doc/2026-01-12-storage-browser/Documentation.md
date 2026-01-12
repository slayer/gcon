# Cloud Storage Bucket Browser - Documentation

## Summary of Changes

This feature adds a Cloud Storage bucket browser to gcon, allowing users to:
- List all GCS buckets in the selected project
- Navigate into buckets to browse objects and folders
- Navigate through folder hierarchies
- Paginate through large result sets

## Technical Details

### Architecture

The implementation follows the existing patterns in the codebase:

```
┌─────────────────────────────────────────────────────────────┐
│                         App (app.go)                        │
│  - Routes messages and manages view lifecycle               │
│  - Handles BucketSelectedMsg to switch to ObjectsView       │
└─────────────────────────────────────────────────────────────┘
           │                    │
           ▼                    ▼
┌─────────────────┐    ┌─────────────────┐
│   BucketsView   │───▶│   ObjectsView   │
│  (buckets.go)   │    │  (objects.go)   │
│                 │    │                 │
│ - Lists buckets │    │ - Lists objects │
│ - Filtering     │    │ - Folder nav    │
│ - Enter to open │    │ - Pagination    │
└─────────────────┘    └─────────────────┘
           │                    │
           ▼                    ▼
┌─────────────────────────────────────────────────────────────┐
│                    StorageClient (storage.go)               │
│  - ListBuckets(ctx, projectID) -> []Bucket                  │
│  - ListObjects(ctx, bucket, prefix, token, size)            │
└─────────────────────────────────────────────────────────────┘
```

### Key Components

#### StorageClient (`internal/gcp/storage.go`)
- Uses `cloud.google.com/go/storage` SDK
- Provides `ListBuckets()` and `ListObjects()` methods
- Handles pagination with tokens
- Uses "/" delimiter for folder-like navigation

#### BucketsView (`internal/ui/views/buckets.go`)
- Displays bucket list with location, storage class, and creation date
- Supports filtering via "/" key
- Emits `BucketSelectedMsg` when user presses Enter

#### ObjectsView (`internal/ui/views/objects.go`)
- Displays objects and folders within a bucket
- Maintains navigation state with `prefixStack` for back navigation
- Supports pagination (100 items per page)
- Shows folder vs file differentiation with icons

### Navigation Flow

```
Projects ─(sidebar)─▶ Buckets ─(Enter)─▶ Objects ─(Enter folder)─▶ deeper folder
            ▲                     │                    │
            └────────(ESC)────────┴────────(ESC)───────┘
```

**ESC key behavior in ObjectsView:**
1. If in a subfolder → navigate to parent folder
2. If at bucket root → return to BucketsView

### Key Bindings

| View | Key | Action |
|------|-----|--------|
| Buckets | Enter | Browse selected bucket |
| Buckets | r | Refresh bucket list |
| Buckets | / | Filter buckets |
| Objects | Enter | Open folder (or select file) |
| Objects | ESC | Go back (parent folder or buckets) |
| Objects | r | Refresh current view |
| Objects | n | Next page |
| Objects | p | Previous page |
| Objects | / | Filter objects |

### Data Structures

```go
type Bucket struct {
    Name         string
    Location     string
    StorageClass string
    Created      time.Time
}

type StorageObject struct {
    Name        string    // Full path
    DisplayName string    // Just filename
    Size        int64
    Updated     time.Time
    ContentType string
    IsFolder    bool
}
```

## Testing

Run tests:
```bash
make test
```

Run linter:
```bash
make lint
```

## Manual Testing Checklist

1. Navigate to Buckets via sidebar (Tab to focus sidebar, navigate to Cloud Storage > Buckets)
2. Verify buckets load and display correctly with location, storage class, creation date
3. Use "/" to filter buckets
4. Press Enter to enter a bucket
5. Verify objects and folders display correctly
6. Press Enter on a folder to navigate into it
7. Verify header shows bucket name and current path
8. Press ESC to go back to parent folder
9. Press ESC at bucket root to return to bucket list
10. Test pagination with n/p keys on buckets with many objects
11. Test refresh with r key

## Future Enhancements

- File download functionality
- File upload functionality
- Object details view
- Bucket creation/deletion
- Object metadata editing
- Signed URL generation
