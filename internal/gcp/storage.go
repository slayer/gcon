package gcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
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
		if errors.Is(err, iterator.Done) {
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
		if errors.Is(err, iterator.Done) {
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
	defer func() { _ = reader.Close() }() //nolint:errcheck // Best-effort cleanup

	// Ensure parent directory exists
	if dir := filepath.Dir(localPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer func() { _ = file.Close() }() //nolint:errcheck // Best-effort cleanup

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
	defer func() { _ = file.Close() }() //nolint:errcheck // Best-effort cleanup

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
			_ = writer.Close() //nolint:errcheck // Best-effort close to avoid resource leak
			return fmt.Errorf("failed to upload: %w", err)
		}
	} else {
		if _, err := io.Copy(writer, file); err != nil {
			_ = writer.Close() //nolint:errcheck // Best-effort close to avoid resource leak
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
		if errors.Is(err, iterator.Done) {
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

// ObjectMetadata contains full metadata for a GCS object
type ObjectMetadata struct {
	// Basic info
	Name        string
	Bucket      string
	Size        int64
	ContentType string
	Created     time.Time
	Updated     time.Time

	// Storage
	StorageClass string

	// Technical details
	MD5Hash        string // Hex encoded
	CRC32C         uint32
	Etag           string
	Generation     int64
	Metageneration int64

	// Access control
	Owner string

	// Cache/content headers
	CacheControl       string
	ContentDisposition string
	ContentEncoding    string
	ContentLanguage    string

	// Custom metadata
	CustomMetadata map[string]string

	// Computed URLs
	PublicURL        string // https://storage.googleapis.com/{bucket}/{object}
	AuthenticatedURL string // https://storage.cloud.google.com/{bucket}/{object}
	GsutilURI        string // gs://{bucket}/{object}
}

// GetObjectMetadata fetches full metadata for an object
func (c *StorageClient) GetObjectMetadata(ctx context.Context, bucketName, objectName string) (*ObjectMetadata, error) {
	attrs, err := c.client.Bucket(bucketName).Object(objectName).Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	// Extract owner email if available
	owner := ""
	if attrs.Owner != "" {
		owner = attrs.Owner
	}

	// Build MD5 hash as hex string
	var md5Hash string
	if len(attrs.MD5) > 0 {
		md5Hash = fmt.Sprintf("%x", attrs.MD5)
	}

	// URL-encode the object name path segments for valid URLs
	// We encode each path segment separately to preserve "/" as path separators
	encodedObjectName := encodeObjectPath(objectName)

	return &ObjectMetadata{
		Name:               attrs.Name,
		Bucket:             attrs.Bucket,
		Size:               attrs.Size,
		ContentType:        attrs.ContentType,
		Created:            attrs.Created,
		Updated:            attrs.Updated,
		StorageClass:       attrs.StorageClass,
		MD5Hash:            md5Hash,
		CRC32C:             attrs.CRC32C,
		Etag:               attrs.Etag,
		Generation:         attrs.Generation,
		Metageneration:     attrs.Metageneration,
		Owner:              owner,
		CacheControl:       attrs.CacheControl,
		ContentDisposition: attrs.ContentDisposition,
		ContentEncoding:    attrs.ContentEncoding,
		ContentLanguage:    attrs.ContentLanguage,
		CustomMetadata:     attrs.Metadata,
		PublicURL:          fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, encodedObjectName),
		AuthenticatedURL:   fmt.Sprintf("https://storage.cloud.google.com/%s/%s", bucketName, encodedObjectName),
		GsutilURI:          fmt.Sprintf("gs://%s/%s", bucketName, objectName),
	}, nil
}

// encodeObjectPath URL-encodes each segment of an object path while preserving "/" separators
func encodeObjectPath(objectName string) string {
	segments := strings.Split(objectName, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return strings.Join(segments, "/")
}

// BucketCreateConfig contains settings for creating a new GCS bucket
type BucketCreateConfig struct {
	Name                   string            // Globally unique bucket name
	Location               string            // Region, dual-region, or multi-region
	LocationType           string            // "region", "dual-region", "multi-region"
	StorageClass           string            // STANDARD, NEARLINE, COLDLINE, ARCHIVE
	PublicAccessPrevention bool              // Enforce public access prevention
	UniformBucketAccess    bool              // true=uniform ACL, false=fine-grained
	VersioningEnabled      bool              // Enable object versioning
	RetentionDays          int               // Object retention period (0=disabled)
	SoftDeleteDays         int               // Soft delete retention (0-90, default 7)
	Labels                 map[string]string // User-defined labels
	EncryptionKey          string            // CMEK resource path (optional)
}

// GCS location constants
var (
	// GCSMultiRegions contains multi-region location codes
	GCSMultiRegions = []string{"US", "EU", "ASIA"}

	// GCSRegions contains single-region location codes
	GCSRegions = []string{
		"us-central1", "us-east1", "us-east4", "us-east5", "us-south1", "us-west1", "us-west2", "us-west3", "us-west4",
		"northamerica-northeast1", "northamerica-northeast2", "southamerica-east1", "southamerica-west1",
		"europe-central2", "europe-north1", "europe-southwest1", "europe-west1", "europe-west2", "europe-west3",
		"europe-west4", "europe-west6", "europe-west8", "europe-west9", "europe-west10", "europe-west12",
		"asia-east1", "asia-east2", "asia-northeast1", "asia-northeast2", "asia-northeast3",
		"asia-south1", "asia-south2", "asia-southeast1", "asia-southeast2",
		"australia-southeast1", "australia-southeast2",
		"me-central1", "me-central2", "me-west1",
		"africa-south1",
	}

	// GCSDualRegions contains dual-region location codes
	GCSDualRegions = []string{"nam4", "eur4", "asia1"}

	// GCSStorageClasses contains available storage classes
	GCSStorageClasses = []string{"STANDARD", "NEARLINE", "COLDLINE", "ARCHIVE"}
)

// GetLocationsForType returns location options for a given location type
func GetLocationsForType(locationType string) []string {
	switch locationType {
	case "region":
		return GCSRegions
	case "dual-region":
		return GCSDualRegions
	case "multi-region":
		return GCSMultiRegions
	default:
		return GCSRegions
	}
}

// CreateBucket creates a new GCS bucket with the specified configuration
func (c *StorageClient) CreateBucket(ctx context.Context, projectID string, config BucketCreateConfig) error {
	bucket := c.client.Bucket(config.Name)

	attrs := &storage.BucketAttrs{
		Location:     config.Location,
		StorageClass: config.StorageClass,
		Labels:       config.Labels,
	}

	// Set uniform bucket-level access
	attrs.UniformBucketLevelAccess = storage.UniformBucketLevelAccess{
		Enabled: config.UniformBucketAccess,
	}

	// Set public access prevention
	if config.PublicAccessPrevention {
		attrs.PublicAccessPrevention = storage.PublicAccessPreventionEnforced
	}

	// Set versioning
	if config.VersioningEnabled {
		attrs.VersioningEnabled = true
	}

	// Set retention policy if specified
	if config.RetentionDays > 0 {
		attrs.RetentionPolicy = &storage.RetentionPolicy{
			RetentionPeriod: time.Duration(config.RetentionDays) * 24 * time.Hour,
		}
	}

	// Set soft delete policy
	if config.SoftDeleteDays > 0 {
		attrs.SoftDeletePolicy = &storage.SoftDeletePolicy{
			RetentionDuration: time.Duration(config.SoftDeleteDays) * 24 * time.Hour,
		}
	}

	// Set CMEK encryption if specified
	if config.EncryptionKey != "" {
		attrs.Encryption = &storage.BucketEncryption{
			DefaultKMSKeyName: config.EncryptionKey,
		}
	}

	if err := bucket.Create(ctx, projectID, attrs); err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	return nil
}

// GetObjectContent downloads object content up to maxBytes
// Returns the content bytes and an error if any
func (c *StorageClient) GetObjectContent(ctx context.Context, bucketName, objectName string, maxBytes int64) ([]byte, error) {
	obj := c.client.Bucket(bucketName).Object(objectName)

	// Create a range reader to read up to maxBytes
	// NewRangeReader handles size limits internally - it reads from offset 0 up to length bytes
	reader, err := obj.NewRangeReader(ctx, 0, maxBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer func() { _ = reader.Close() }() //nolint:errcheck // Best-effort cleanup

	// ReadAll will read up to the smaller of maxBytes or actual object size
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read object content: %w", err)
	}

	return content, nil
}
