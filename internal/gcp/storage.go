package gcp

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// ProgressFunc is a callback for reporting transfer progress
type ProgressFunc func(bytesTransferred, totalBytes int64)

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

	// Let iterator handle pagination via MaxSize - this ensures NextToken is properly set
	if pageSize > 0 {
		it.PageInfo().MaxSize = pageSize
	}

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
	}

	// Check for next page - token is valid after iterator exhausts current page
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

// GetObjectSize returns the size of an object in bytes
func (c *StorageClient) GetObjectSize(ctx context.Context, bucketName, objectName string) (int64, error) {
	attrs, err := c.client.Bucket(bucketName).Object(objectName).Attrs(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get object attributes: %w", err)
	}
	return attrs.Size, nil
}

// DownloadObject downloads an object from GCS to a local file
func (c *StorageClient) DownloadObject(ctx context.Context, bucketName, objectName, localPath string, progress ProgressFunc) error {
	// Get object attributes to know total size
	attrs, err := c.client.Bucket(bucketName).Object(objectName).Attrs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get object attributes: %w", err)
	}
	totalSize := attrs.Size

	// Create the reader
	reader, err := c.client.Bucket(bucketName).Object(objectName).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to create reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Ensure parent directory exists
	if dir := filepath.Dir(localPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Copy with progress reporting (copyWithProgress handles final progress update internally)
	if progress != nil {
		if _, err := copyWithProgress(file, reader, totalSize, progress); err != nil {
			return fmt.Errorf("failed to download: %w", err)
		}
	} else {
		if _, err := io.Copy(file, reader); err != nil {
			return fmt.Errorf("failed to download: %w", err)
		}
	}

	return nil
}

// UploadObject uploads a local file to GCS
func (c *StorageClient) UploadObject(ctx context.Context, bucketName, objectName, localPath string, progress ProgressFunc) error {
	// Open local file
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Get file size for progress reporting
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	totalSize := stat.Size()

	// Create writer
	writer := c.client.Bucket(bucketName).Object(objectName).NewWriter(ctx)

	// Copy with progress reporting (copyWithProgress handles final progress update internally)
	if progress != nil {
		if _, err := copyWithProgress(writer, file, totalSize, progress); err != nil {
			_ = writer.Close() // Best-effort close to avoid resource leak
			return fmt.Errorf("failed to upload: %w", err)
		}
	} else {
		if _, err := io.Copy(writer, file); err != nil {
			_ = writer.Close() // Best-effort close to avoid resource leak
			return fmt.Errorf("failed to upload: %w", err)
		}
	}

	// Close the writer to finalize the upload
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to finalize upload: %w", err)
	}

	return nil
}

// ListAllObjects returns all objects under a prefix (for recursive download)
func (c *StorageClient) ListAllObjects(ctx context.Context, bucketName, prefix string) ([]StorageObject, error) {
	var objects []StorageObject

	bucket := c.client.Bucket(bucketName)
	query := &storage.Query{
		Prefix: prefix,
		// No delimiter = get all objects recursively
	}

	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Skip "folder" markers (objects ending with /)
		if strings.HasSuffix(attrs.Name, "/") {
			continue
		}

		objects = append(objects, objectFromAttrs(attrs, prefix))
	}

	return objects, nil
}

// progressWriter wraps an io.Writer to report progress
type progressWriter struct {
	writer      io.Writer
	totalSize   int64
	written     int64
	progress    ProgressFunc
	lastReport  int64
	reportEvery int64 // Report every N bytes
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	pw.written += int64(n)

	// Report progress periodically to avoid too many updates
	if pw.progress != nil && (pw.written-pw.lastReport >= pw.reportEvery || pw.written >= pw.totalSize) {
		pw.progress(pw.written, pw.totalSize)
		pw.lastReport = pw.written
	}

	return n, err
}

// copyWithProgress copies from src to dst with progress reporting
func copyWithProgress(dst io.Writer, src io.Reader, totalSize int64, progress ProgressFunc) (int64, error) {
	// Report every 1% or 64KB, whichever is larger
	reportEvery := totalSize / 100
	if reportEvery < 64*1024 {
		reportEvery = 64 * 1024
	}

	pw := &progressWriter{
		writer:      dst,
		totalSize:   totalSize,
		progress:    progress,
		reportEvery: reportEvery,
	}

	written, err := io.Copy(pw, src)
	return written, err
}

// DeleteObject deletes a single object from GCS
func (c *StorageClient) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	obj := c.client.Bucket(bucketName).Object(objectName)
	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object %s: %w", objectName, err)
	}
	return nil
}
