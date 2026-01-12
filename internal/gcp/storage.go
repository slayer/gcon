package gcp

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// Bucket represents a GCS bucket with simplified metadata
type Bucket struct {
	Name         string
	Location     string
	StorageClass string
	Created      time.Time
}

// StorageObject represents a GCS object (file or virtual folder)
type StorageObject struct {
	Name        string    // Full object path (e.g., "folder/file.txt")
	DisplayName string    // Just the filename or folder name
	Size        int64     // Size in bytes (0 for folders)
	Updated     time.Time // Last modification time
	ContentType string    // MIME type
	IsFolder    bool      // True for virtual folders (prefixes)
}

// ObjectListResult contains paginated object listing results
type ObjectListResult struct {
	Objects   []StorageObject
	NextToken string // Page token for next page (empty if no more)
	HasMore   bool   // True if there are more results
}

// StorageClient handles Cloud Storage operations
type StorageClient struct {
	client *storage.Client
}

// NewStorageClient creates a new Cloud Storage client
func NewStorageClient(ctx context.Context) (*StorageClient, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	return &StorageClient{client: client}, nil
}

// Close closes the storage client
func (c *StorageClient) Close() error {
	return c.client.Close()
}

// ListBuckets returns all buckets in a project
func (c *StorageClient) ListBuckets(ctx context.Context, projectID string) ([]Bucket, error) {
	var buckets []Bucket

	it := c.client.Buckets(ctx, projectID)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list buckets: %w", err)
		}

		buckets = append(buckets, bucketFromAttrs(attrs))
	}

	return buckets, nil
}

// ListObjects returns objects in a bucket with optional prefix filtering and pagination
func (c *StorageClient) ListObjects(ctx context.Context, bucketName, prefix, pageToken string, pageSize int) (*ObjectListResult, error) {
	result := &ObjectListResult{
		Objects: make([]StorageObject, 0),
	}

	bucket := c.client.Bucket(bucketName)
	query := &storage.Query{
		Prefix:    prefix,
		Delimiter: "/", // Use delimiter for folder-like navigation
	}

	it := bucket.Objects(ctx, query)

	// Set page token if provided
	if pageToken != "" {
		it.PageInfo().Token = pageToken
	}

	// Set page size
	if pageSize > 0 {
		it.PageInfo().MaxSize = pageSize
	}

	count := 0
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Handle prefixes (virtual folders)
		if attrs.Prefix != "" {
			result.Objects = append(result.Objects, StorageObject{
				Name:        attrs.Prefix,
				DisplayName: extractFolderName(attrs.Prefix),
				IsFolder:    true,
			})
		} else {
			// Regular object
			result.Objects = append(result.Objects, objectFromAttrs(attrs, prefix))
		}

		count++
		if pageSize > 0 && count >= pageSize {
			break
		}
	}

	// Check for next page
	result.NextToken = it.PageInfo().Token
	result.HasMore = result.NextToken != ""

	return result, nil
}

// bucketFromAttrs converts storage.BucketAttrs to our Bucket struct
func bucketFromAttrs(attrs *storage.BucketAttrs) Bucket {
	return Bucket{
		Name:         attrs.Name,
		Location:     attrs.Location,
		StorageClass: attrs.StorageClass,
		Created:      attrs.Created,
	}
}

// objectFromAttrs converts storage.ObjectAttrs to our StorageObject struct
func objectFromAttrs(attrs *storage.ObjectAttrs, prefix string) StorageObject {
	// Extract display name (filename without path prefix)
	displayName := attrs.Name
	if prefix != "" {
		displayName = strings.TrimPrefix(attrs.Name, prefix)
	}
	// If still has path separators, get just the filename
	if idx := strings.LastIndex(displayName, "/"); idx != -1 {
		displayName = displayName[idx+1:]
	}

	return StorageObject{
		Name:        attrs.Name,
		DisplayName: displayName,
		Size:        attrs.Size,
		Updated:     attrs.Updated,
		ContentType: attrs.ContentType,
		IsFolder:    false,
	}
}

// extractFolderName extracts the folder name from a prefix path
// e.g., "folder1/folder2/" -> "folder2"
func extractFolderName(prefix string) string {
	// Remove trailing slash
	trimmed := strings.TrimSuffix(prefix, "/")
	// Get base name
	return path.Base(trimmed)
}

// FormatSize returns a human-readable size string
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
