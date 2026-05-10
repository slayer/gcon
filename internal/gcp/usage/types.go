// Package usage provides bucket-level storage usage analysis: monitoring-based
// totals (fast, daily-stale) and on-demand deep scans (real-time, with breakdowns
// by storage class, top-level prefix, and file extension).
//
// All public methods are safe for concurrent use; the Scanner is intended to be
// constructed once and shared across views.
package usage

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/slayer/gcon/internal/gcp"
)

// Source identifies how a BucketUsage record was produced.
type Source int

const (
	// SourceMonitoring means the totals came from Cloud Monitoring metrics.
	// Breakdown maps will be empty.
	SourceMonitoring Source = iota
	// SourceDeepScan means the totals came from listing every object in the
	// (bucket, prefix). Breakdown maps are populated.
	SourceDeepScan
)

// Stat holds aggregate bytes and object count for a single bucket-or-folder slice.
type Stat struct {
	Bytes int64
	Count int64
}

// BucketUsage is the complete usage record for a (bucket, prefix) pair.
// For SourceMonitoring records, Prefix is always "" and breakdown maps are nil.
type BucketUsage struct {
	Bucket         string
	Prefix         string
	TotalBytes     int64
	ObjectCount    int64
	ByStorageClass map[string]Stat // populated only when Source == SourceDeepScan
	ByTopPrefix    map[string]Stat // populated only when Source == SourceDeepScan
	ByExtension    map[string]Stat // populated only when Source == SourceDeepScan
	Source         Source
	ScannedAt      time.Time
}

// ProgressMsg is emitted periodically during a deep scan to surface live counts.
// The receiving view should match by Bucket+Prefix; the App uses JobID to update
// the footer task.
type ProgressMsg struct {
	JobID          string
	Bucket         string
	Prefix         string
	ObjectsScanned int64
	BytesScanned   int64
}

// ReadyMsg is emitted exactly once per scan (or monitoring fetch) when results
// are available or an error occurred. After ReadyMsg, no more Progress messages
// will arrive for this JobID.
type ReadyMsg struct {
	JobID string
	Usage BucketUsage // valid only when Err == nil
	Err   error
}

// StorageLister is the subset of *gcp.StorageClient used by the scanner.
// Defined as an interface so tests can supply canned object pages without GCP.
type StorageLister interface {
	ListAllObjects(ctx context.Context, bucket, prefix string) ([]gcp.StorageObject, error)
}

// MonitoringFetcher is the subset of *gcp.MonitoringClient used by the scanner.
type MonitoringFetcher interface {
	FetchBucketUsage(ctx context.Context, bucket string) (bytes, count int64, asOf time.Time, err error)
}

// noOpCmd is a tea.Cmd that emits no message. Used when a public method has
// nothing to do (cache hit, unknown jobID, etc.) but the API requires a Cmd.
func noOpCmd() tea.Cmd {
	return func() tea.Msg { return nil }
}
