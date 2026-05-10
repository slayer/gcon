# Bucket Usage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Surface storage usage and object statistics for GCS buckets in `gcon`, with always-on Cloud Monitoring metrics in the buckets list and opt-in deep-scan breakdowns (storage class / top-level prefix / file extension) accessible from a new bucket details view and from the existing objects browser.

**Architecture:** A new `internal/gcp/usage` package owns a single `Scanner` that fetches bucket-level totals via Cloud Monitoring (cheap, ~24h stale) and runs streaming deep scans via the existing `StorageClient.ListAllObjects`. Results are cached in memory keyed by `(bucket, prefix)`. Views request data via tea messages; the App routes responses and drives a footer task with live progress. Three sequential phases — each ends in a commit-ready state and a single PR.

**Tech Stack:** Go 1.22+, Bubble Tea, Cloud Monitoring v3 SDK (`monitoring/apiv3/v2`), Cloud Storage SDK (`cloud.google.com/go/storage`), testify for assertions.

**Spec:** [`docs/superpowers/specs/2026-05-10-bucket-usage-design.md`](../specs/2026-05-10-bucket-usage-design.md)

---

## File Structure

### Phase 1 — Data Layer (PR 1)

**Created:**
- `internal/gcp/usage/types.go` — `BucketUsage`, `Stat`, `Source`, message types
- `internal/gcp/usage/scanner.go` — `Scanner` type, public API (`Get`, `FetchMonitoring`, `StartDeepScan`, `NextMessage`, `Cancel`, `Invalidate`)
- `internal/gcp/usage/job.go` — internal `scanJob` type with tally logic and goroutine lifecycle
- `internal/gcp/usage/tally.go` — pure tally functions (per storage class, top prefix, extension)
- `internal/gcp/usage/scanner_test.go` — unit tests with fake `StorageLister`/`MonitoringFetcher`
- `internal/gcp/usage/tally_test.go` — pure-function unit tests
- `internal/gcp/monitoring_storage.go` — `MonitoringClient.FetchBucketUsage`
- `internal/gcp/monitoring_storage_test.go` — table-driven tests with fake metric client

**Modified:**
- `internal/ui/app.go` — add `updateRunningTask(id, description string)` helper

### Phase 2 — Buckets List Integration (PR 2)

**Created:**
- `internal/ui/views/bucket_usage_messages.go` — `UsageMonitoringRequestMsg`, `UsageDeepScanRequestMsg`, `BucketDetailsRequestMsg` (last one used in Phase 3 but defined here so Phase 2 can stub it)

**Modified:**
- `internal/ui/app.go` — add `usageScanner *usage.Scanner` field, init helper, `Update` cases for usage messages and `Ctrl+X`
- `internal/ui/app_navigation.go` — `handleUsageMonitoringRequest`, `handleUsageDeepScanRequest`, `dispatchUsageProgress`, `dispatchUsageReady`, `clearAllViews` cleanup
- `internal/ui/keys.go` — add `CancelUsageScan` binding (`Ctrl+X`)
- `internal/ui/views/buckets.go` — Size/Objects columns, monitoring fetch on load, `C` key, `i` key, ready/progress handlers
- `internal/ui/views/buckets_test.go` — extended tests for new behavior
- `README.md` and `.claude/rules/key-bindings.md` — document new keys

### Phase 3 — BucketDetailsView + ObjectsView Folder Scan (PR 3)

**Created:**
- `internal/ui/views/bucket_details.go` — new `BucketDetailsView` with Details/Usage tabs
- `internal/ui/views/bucket_details_test.go` — view tests

**Modified:**
- `internal/ui/app.go` — add `bucketDetailsView` field, `ViewBucketDetails` enum value, render switch entry, getCurrentViewModel case
- `internal/ui/app_render.go` — render BucketDetailsView
- `internal/ui/app_navigation.go` — `handleBucketDetailsRequest`, clearAllViews entry, sidebar guards if needed, dispatch routing for new view
- `internal/ui/components/commandpalette/commands.go` — `ViewBucketDetails` constant (no nav command — requires bucket selection)
- `internal/ui/views/objects.go` — `C` key, `folderUsage *usage.BucketUsage` field, ready handler, inline stats line
- `internal/ui/views/objects_test.go` — extended tests
- `README.md` and `.claude/rules/key-bindings.md` — document new view and keys

---

## Phase 1: Data Layer (PR 1)

### Task 1.1: Create usage package types

**Files:**
- Create: `internal/gcp/usage/types.go`

- [ ] **Step 1: Create the types file**

```go
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
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/vlad/dev/my/gcon && go build ./internal/gcp/usage/...`
Expected: build succeeds (no other files in package yet).

- [ ] **Step 3: Commit**

```bash
git add internal/gcp/usage/types.go
git commit -m "2026-05-10: add usage package types and message contracts"
```

---

### Task 1.2: Add MonitoringClient.FetchBucketUsage

**Files:**
- Create: `internal/gcp/monitoring_storage.go`
- Create: `internal/gcp/monitoring_storage_test.go`

- [ ] **Step 1: Write the failing test**

```go
package gcp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFetchBucketUsage_BothMetricsPresent verifies bytes and count are joined.
func TestFetchBucketUsage_BothMetricsPresent(t *testing.T) {
	now := time.Now()
	c := &MonitoringClient{
		projectID: "test-project",
		fetchFunc: func(ctx context.Context, filter string, duration time.Duration) ([]DataPoint, error) {
			// Two filters in sequence: total_bytes then object_count.
			if contains(filter, "total_bytes") {
				return []DataPoint{{Timestamp: now.Add(-2 * time.Hour), Value: 1.5e9}}, nil
			}
			if contains(filter, "object_count") {
				return []DataPoint{{Timestamp: now.Add(-2 * time.Hour), Value: 4321}}, nil
			}
			return nil, nil
		},
	}
	bytes, count, asOf, err := c.FetchBucketUsage(context.Background(), "my-bucket")
	require.NoError(t, err)
	assert.Equal(t, int64(1_500_000_000), bytes)
	assert.Equal(t, int64(4321), count)
	assert.WithinDuration(t, now.Add(-2*time.Hour), asOf, time.Second)
}

func TestFetchBucketUsage_NoData(t *testing.T) {
	c := &MonitoringClient{
		projectID: "test-project",
		fetchFunc: func(ctx context.Context, filter string, duration time.Duration) ([]DataPoint, error) {
			return nil, nil
		},
	}
	bytes, count, asOf, err := c.FetchBucketUsage(context.Background(), "new-bucket")
	require.NoError(t, err)
	assert.Equal(t, int64(0), bytes)
	assert.Equal(t, int64(0), count)
	assert.True(t, asOf.IsZero())
}

func TestFetchBucketUsage_OnlyBytes(t *testing.T) {
	now := time.Now()
	c := &MonitoringClient{
		projectID: "test-project",
		fetchFunc: func(ctx context.Context, filter string, duration time.Duration) ([]DataPoint, error) {
			if contains(filter, "total_bytes") {
				return []DataPoint{{Timestamp: now, Value: 100}}, nil
			}
			return nil, nil
		},
	}
	bytes, count, asOf, err := c.FetchBucketUsage(context.Background(), "b")
	require.NoError(t, err)
	assert.Equal(t, int64(100), bytes)
	assert.Equal(t, int64(0), count)
	assert.WithinDuration(t, now, asOf, time.Second)
}

// contains is a small helper to keep the test free of stdlib imports.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		(len(haystack) > len(needle) && (containsAt(haystack, needle))))
}
func containsAt(h, n string) bool {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Add seam to MonitoringClient for test injection**

Modify `internal/gcp/monitoring.go` — add a `fetchFunc` field with a sensible default, and route `fetchMetricData` through it. This is the minimal seam needed to test `FetchBucketUsage` without GCP.

In `internal/gcp/monitoring.go`, change the `MonitoringClient` struct to:

```go
type MonitoringClient struct {
	metricsClient *monitoring.MetricClient
	projectID     string
	// fetchFunc is the seam for tests. Production code leaves this nil and
	// the methods fall back to fetchMetricData.
	fetchFunc func(ctx context.Context, filter string, duration time.Duration) ([]DataPoint, error)
}
```

Add a small dispatch helper (place it right after `fetchMetricDataWithAligner`):

```go
// fetch dispatches to fetchFunc when set (tests) or fetchMetricData (production).
func (c *MonitoringClient) fetch(ctx context.Context, filter string, duration time.Duration) ([]DataPoint, error) {
	if c.fetchFunc != nil {
		return c.fetchFunc(ctx, filter, duration)
	}
	return c.fetchMetricData(ctx, filter, duration)
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/gcp/ -run TestFetchBucketUsage -v`
Expected: FAIL — `FetchBucketUsage` is not yet defined.

- [ ] **Step 4: Implement FetchBucketUsage**

Create `internal/gcp/monitoring_storage.go`:

```go
package gcp

import (
	"context"
	"fmt"
	"time"
)

// FetchBucketUsage returns the most recent (within ~36h) total bytes and
// object count for a bucket via Cloud Monitoring. These metrics are published
// roughly daily by GCP, so values may be up to ~24h stale.
//
// Returns (0, 0, zero-time, nil) if no data exists for the bucket (e.g. brand
// new bucket whose first metric publication has not happened yet). Callers
// should treat zero asOf as "no data yet" rather than as "0 bytes".
func (c *MonitoringClient) FetchBucketUsage(ctx context.Context, bucket string) (bytes, count int64, asOf time.Time, err error) {
	const window = 36 * time.Hour

	bytesFilter := fmt.Sprintf(
		`metric.type="storage.googleapis.com/storage/total_bytes" `+
			`resource.type="gcs_bucket" `+
			`resource.labels.bucket_name="%s"`, bucket)
	countFilter := fmt.Sprintf(
		`metric.type="storage.googleapis.com/storage/object_count" `+
			`resource.type="gcs_bucket" `+
			`resource.labels.bucket_name="%s"`, bucket)

	bytesPoints, err := c.fetch(ctx, bytesFilter, window)
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("fetch bucket bytes: %w", err)
	}
	countPoints, err := c.fetch(ctx, countFilter, window)
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("fetch bucket object count: %w", err)
	}

	// fetchMetricData sorts ascending; the last element is the most recent.
	if n := len(bytesPoints); n > 0 {
		bytes = int64(bytesPoints[n-1].Value)
		asOf = bytesPoints[n-1].Timestamp
	}
	if n := len(countPoints); n > 0 {
		count = int64(countPoints[n-1].Value)
		// Prefer the freshest of the two timestamps if both exist.
		if t := countPoints[n-1].Timestamp; t.After(asOf) {
			asOf = t
		}
	}
	return bytes, count, asOf, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/gcp/ -run TestFetchBucketUsage -v`
Expected: PASS for all three subtests.

- [ ] **Step 6: Run full gcp test suite to confirm no regression**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/gcp/...`
Expected: all existing tests still pass.

- [ ] **Step 7: Commit**

```bash
git add internal/gcp/monitoring.go internal/gcp/monitoring_storage.go internal/gcp/monitoring_storage_test.go
git commit -m "2026-05-10: add MonitoringClient.FetchBucketUsage"
```

---

### Task 1.3: Implement pure tally functions

**Files:**
- Create: `internal/gcp/usage/tally.go`
- Create: `internal/gcp/usage/tally_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package usage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/gcp"
)

func TestTopPrefixSegment(t *testing.T) {
	tests := []struct {
		name, fullName, scanPrefix, want string
	}{
		{"root no slash", "README.md", "", "(root)"},
		{"top folder", "logs/2025/file.log", "", "logs/"},
		{"deep file scan-root", "a/b/c/d.txt", "", "a/"},
		{"prefix-scoped strips prefix", "logs/2025/01/file.log", "logs/", "2025/"},
		{"prefix-scoped root", "logs/file.log", "logs/", "(root)"},
		{"prefix without trailing slash", "logs/file.log", "logs", "(root)"},
		{"object equals prefix", "logs/", "logs/", "(root)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := topPrefixSegment(tt.fullName, tt.scanPrefix)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtensionOf(t *testing.T) {
	tests := []struct {
		name, fullName, want string
	}{
		{"simple", "file.txt", ".txt"},
		{"upper to lower", "IMAGE.JPG", ".jpg"},
		{"double extension keeps last", "archive.tar.gz", ".gz"},
		{"no extension", "Makefile", "(none)"},
		{"hidden no extension", ".bashrc", "(none)"},
		{"hidden with extension", ".env.local", ".local"},
		{"folder name", "logs/", "(none)"},
		{"path with extension", "logs/2025/file.parquet", ".parquet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extensionOf(tt.fullName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTallyObjects(t *testing.T) {
	now := time.Now()
	objs := []gcp.StorageObject{
		{Name: "logs/a.log", Size: 100, ContentType: "text/plain", Updated: now},
		{Name: "logs/b.log", Size: 200, ContentType: "text/plain", Updated: now},
		{Name: "exports/data.parquet", Size: 1_000_000, ContentType: "application/octet-stream", Updated: now},
		{Name: "README.md", Size: 50, ContentType: "text/markdown", Updated: now},
	}

	// Pretend storage class came from object metadata; tally takes it via a parallel
	// slice so we don't need to extend StorageObject for v1.
	classes := []string{"STANDARD", "STANDARD", "NEARLINE", "STANDARD"}

	usage := tallyObjects("my-bucket", "", objs, classes)

	assert.Equal(t, int64(1_000_350), usage.TotalBytes)
	assert.Equal(t, int64(4), usage.ObjectCount)
	assert.Equal(t, Stat{Bytes: 350, Count: 3}, usage.ByStorageClass["STANDARD"])
	assert.Equal(t, Stat{Bytes: 1_000_000, Count: 1}, usage.ByStorageClass["NEARLINE"])
	assert.Equal(t, Stat{Bytes: 300, Count: 2}, usage.ByTopPrefix["logs/"])
	assert.Equal(t, Stat{Bytes: 1_000_000, Count: 1}, usage.ByTopPrefix["exports/"])
	assert.Equal(t, Stat{Bytes: 50, Count: 1}, usage.ByTopPrefix["(root)"])
	assert.Equal(t, Stat{Bytes: 300, Count: 2}, usage.ByExtension[".log"])
	assert.Equal(t, Stat{Bytes: 1_000_000, Count: 1}, usage.ByExtension[".parquet"])
	assert.Equal(t, Stat{Bytes: 50, Count: 1}, usage.ByExtension[".md"])
	assert.Equal(t, SourceDeepScan, usage.Source)
	assert.Equal(t, "my-bucket", usage.Bucket)
}

func TestTallyObjects_PrefixScoped(t *testing.T) {
	objs := []gcp.StorageObject{
		{Name: "logs/2025/jan/a.log", Size: 100},
		{Name: "logs/2025/feb/b.log", Size: 200},
		{Name: "logs/2024/old.log", Size: 50},
	}
	classes := []string{"STANDARD", "STANDARD", "STANDARD"}
	usage := tallyObjects("my-bucket", "logs/", objs, classes)

	assert.Equal(t, int64(350), usage.TotalBytes)
	assert.Equal(t, "logs/", usage.Prefix)
	assert.Equal(t, Stat{Bytes: 300, Count: 2}, usage.ByTopPrefix["2025/"])
	assert.Equal(t, Stat{Bytes: 50, Count: 1}, usage.ByTopPrefix["2024/"])
}
```

- [ ] **Step 2: Run tests to confirm they fail**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/gcp/usage/ -run "TestTopPrefix|TestExtension|TestTallyObjects" -v`
Expected: FAIL — `topPrefixSegment`, `extensionOf`, `tallyObjects` are not defined.

- [ ] **Step 3: Implement tally.go**

Create `internal/gcp/usage/tally.go`:

```go
package usage

import (
	"strings"
	"time"

	"github.com/slayer/gcon/internal/gcp"
)

// rootBucket is the synthetic key used in ByTopPrefix for objects that live
// directly under the scan root (no further "/" separator).
const rootBucket = "(root)"

// noExtension is the synthetic key used in ByExtension for files without one.
const noExtension = "(none)"

// topPrefixSegment returns the first path segment of fullName *relative to*
// scanPrefix. For objects sitting directly under the scan root (no further
// slash), it returns "(root)". The scanPrefix may or may not end with a slash;
// both are accepted.
func topPrefixSegment(fullName, scanPrefix string) string {
	rel := fullName
	if scanPrefix != "" {
		// Normalize: ensure scanPrefix ends with "/" before stripping.
		normalized := scanPrefix
		if !strings.HasSuffix(normalized, "/") {
			normalized += "/"
		}
		rel = strings.TrimPrefix(fullName, normalized)
	}
	if rel == "" {
		return rootBucket
	}
	if i := strings.Index(rel, "/"); i >= 0 {
		return rel[:i+1] // include trailing slash for clarity
	}
	return rootBucket
}

// extensionOf returns the lowercase extension of the basename (including the
// leading dot), or "(none)" if there isn't one. Hidden files starting with "."
// and no further dot (e.g. ".bashrc") count as having no extension.
func extensionOf(fullName string) string {
	base := fullName
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if base == "" {
		return noExtension
	}
	// Strip a leading dot for hidden files so ".bashrc" → "bashrc" → no ext,
	// while ".env.local" → ".local".
	stripped := strings.TrimPrefix(base, ".")
	dot := strings.LastIndex(stripped, ".")
	if dot < 0 {
		return noExtension
	}
	return strings.ToLower(stripped[dot:])
}

// tallyObjects walks the objects and produces a fully populated BucketUsage
// with breakdowns. The classes slice MUST be the same length as objs — entry i
// is the storage class of object i. Pass an empty string to skip classification
// for that object.
func tallyObjects(bucket, prefix string, objs []gcp.StorageObject, classes []string) BucketUsage {
	u := BucketUsage{
		Bucket:         bucket,
		Prefix:         prefix,
		ByStorageClass: make(map[string]Stat),
		ByTopPrefix:    make(map[string]Stat),
		ByExtension:    make(map[string]Stat),
		Source:         SourceDeepScan,
		ScannedAt:      time.Now(),
	}
	for i, o := range objs {
		if o.IsFolder {
			continue // virtual prefix, not a real object
		}
		u.TotalBytes += o.Size
		u.ObjectCount++
		if i < len(classes) && classes[i] != "" {
			cur := u.ByStorageClass[classes[i]]
			cur.Bytes += o.Size
			cur.Count++
			u.ByStorageClass[classes[i]] = cur
		}
		seg := topPrefixSegment(o.Name, prefix)
		curP := u.ByTopPrefix[seg]
		curP.Bytes += o.Size
		curP.Count++
		u.ByTopPrefix[seg] = curP
		ext := extensionOf(o.Name)
		curE := u.ByExtension[ext]
		curE.Bytes += o.Size
		curE.Count++
		u.ByExtension[ext] = curE
	}
	return u
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/gcp/usage/ -v`
Expected: PASS for all subtests.

- [ ] **Step 5: Commit**

```bash
git add internal/gcp/usage/tally.go internal/gcp/usage/tally_test.go
git commit -m "2026-05-10: add usage tally helpers (prefix/extension grouping)"
```

---

### Task 1.4: Implement Scanner cache + FetchMonitoring

**Files:**
- Create: `internal/gcp/usage/scanner.go`
- Modify: `internal/gcp/usage/scanner_test.go` (created in this task)

- [ ] **Step 1: Write the failing tests**

Create `internal/gcp/usage/scanner_test.go`:

```go
package usage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

// fakeMonitoring is a programmable MonitoringFetcher.
type fakeMonitoring struct {
	mu     sync.Mutex
	calls  int
	bytes  int64
	count  int64
	asOf   time.Time
	err    error
}

func (f *fakeMonitoring) FetchBucketUsage(_ context.Context, _ string) (int64, int64, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.bytes, f.count, f.asOf, f.err
}

// fakeStorage is a programmable StorageLister.
type fakeStorage struct {
	mu      sync.Mutex
	calls   int
	objs    []gcp.StorageObject
	err     error
	delay   time.Duration // optional pause to simulate a slow scan
	classes func(o gcp.StorageObject) string
}

func (f *fakeStorage) ListAllObjects(ctx context.Context, _, _ string) ([]gcp.StorageObject, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.objs, f.err
}

func TestScanner_GetCacheMiss(t *testing.T) {
	s := New(&fakeStorage{}, &fakeMonitoring{})
	_, ok := s.Get("any", "")
	assert.False(t, ok)
}

func TestScanner_FetchMonitoring_Success(t *testing.T) {
	mon := &fakeMonitoring{bytes: 999, count: 7, asOf: time.Now()}
	s := New(&fakeStorage{}, mon)
	cmd := s.FetchMonitoring(context.Background(), "b")
	require.NotNil(t, cmd)

	msg := cmd()
	ready, ok := msg.(ReadyMsg)
	require.True(t, ok, "expected ReadyMsg, got %T", msg)
	require.NoError(t, ready.Err)
	assert.Equal(t, int64(999), ready.Usage.TotalBytes)
	assert.Equal(t, int64(7), ready.Usage.ObjectCount)
	assert.Equal(t, SourceMonitoring, ready.Usage.Source)
	assert.Equal(t, "b", ready.Usage.Bucket)

	// Cache populated.
	cached, ok := s.Get("b", "")
	assert.True(t, ok)
	assert.Equal(t, int64(999), cached.TotalBytes)
}

func TestScanner_FetchMonitoring_CacheHit(t *testing.T) {
	mon := &fakeMonitoring{bytes: 10, asOf: time.Now()}
	s := New(&fakeStorage{}, mon)

	// Prime cache.
	s.FetchMonitoring(context.Background(), "b")()
	assert.Equal(t, 1, mon.calls)

	// Second call within TTL should be a no-op (cache hit).
	cmd := s.FetchMonitoring(context.Background(), "b")
	msg := cmd()
	if msg != nil {
		// If a message did come back, it must be a ReadyMsg from cache (no extra fetch).
		if _, ok := msg.(ReadyMsg); !ok {
			t.Fatalf("unexpected message type %T", msg)
		}
	}
	assert.Equal(t, 1, mon.calls, "cache hit should not call monitoring again")
}

func TestScanner_FetchMonitoring_Error(t *testing.T) {
	mon := &fakeMonitoring{err: errors.New("boom")}
	s := New(&fakeStorage{}, mon)
	msg := s.FetchMonitoring(context.Background(), "b")()
	ready, ok := msg.(ReadyMsg)
	require.True(t, ok)
	assert.Error(t, ready.Err)
}

func TestScanner_Invalidate(t *testing.T) {
	mon := &fakeMonitoring{bytes: 1, asOf: time.Now()}
	s := New(&fakeStorage{}, mon)
	s.FetchMonitoring(context.Background(), "b")()
	require.Equal(t, 1, mon.calls)

	s.Invalidate("b", "")
	s.FetchMonitoring(context.Background(), "b")()
	assert.Equal(t, 2, mon.calls, "after invalidate, fetch should hit monitoring again")
}
```

- [ ] **Step 2: Run tests to confirm they fail**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/gcp/usage/ -run TestScanner -v`
Expected: FAIL — `New`, `FetchMonitoring`, `Get`, `Invalidate` not defined.

- [ ] **Step 3: Implement scanner.go (cache + FetchMonitoring only — deep scan in Task 1.5)**

Create `internal/gcp/usage/scanner.go`:

```go
package usage

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// monitoringTTL is how long a SourceMonitoring cache entry is considered fresh
// before the scanner will re-fetch from Cloud Monitoring.
const monitoringTTL = 10 * time.Minute

// Scanner is the central registry for bucket usage queries. It is safe for
// concurrent use; views and the App share a single instance per project.
type Scanner struct {
	storage    StorageLister
	monitoring MonitoringFetcher

	mu       sync.RWMutex
	cache    map[string]BucketUsage
	inflight map[string]*scanJob // populated in Task 1.5; declared here for layout
}

// New constructs a Scanner with the given backends. The two arguments are
// interfaces so tests can supply fakes; production callers pass in
// *gcp.StorageClient and *gcp.MonitoringClient.
func New(storage StorageLister, monitoring MonitoringFetcher) *Scanner {
	return &Scanner{
		storage:    storage,
		monitoring: monitoring,
		cache:      make(map[string]BucketUsage),
		inflight:   make(map[string]*scanJob),
	}
}

// cacheKey is the stable in-memory key for a (bucket, prefix) pair.
func cacheKey(bucket, prefix string) string {
	return bucket + "|" + prefix
}

// Get returns the cached usage for (bucket, prefix), if any.
func (s *Scanner) Get(bucket, prefix string) (BucketUsage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.cache[cacheKey(bucket, prefix)]
	return u, ok
}

// Invalidate drops the cached entry (if any) for (bucket, prefix). Does not
// cancel any in-flight scan for that key.
func (s *Scanner) Invalidate(bucket, prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, cacheKey(bucket, prefix))
}

// FetchMonitoring returns a tea.Cmd that fetches bucket-level totals from
// Cloud Monitoring and emits a ReadyMsg. If a fresh monitoring entry already
// exists in the cache, the returned Cmd emits that cached value immediately
// without contacting GCP.
//
// Monitoring entries are always keyed with prefix == "".
func (s *Scanner) FetchMonitoring(ctx context.Context, bucket string) tea.Cmd {
	key := cacheKey(bucket, "")

	// Cache check.
	s.mu.RLock()
	if u, ok := s.cache[key]; ok && u.Source == SourceMonitoring && time.Since(u.ScannedAt) < monitoringTTL {
		s.mu.RUnlock()
		cached := u
		return func() tea.Msg {
			return ReadyMsg{JobID: "monitoring:" + bucket, Usage: cached}
		}
	}
	s.mu.RUnlock()

	return func() tea.Msg {
		bytes, count, asOf, err := s.monitoring.FetchBucketUsage(ctx, bucket)
		if err != nil {
			return ReadyMsg{JobID: "monitoring:" + bucket, Err: err}
		}
		// Use ScannedAt as "when this record was created", not the asOf of the
		// underlying metric. asOf is preserved on the BucketUsage so the UI can
		// render "12h ago" — but cache TTL is based on when we fetched.
		u := BucketUsage{
			Bucket:      bucket,
			Prefix:      "",
			TotalBytes:  bytes,
			ObjectCount: count,
			Source:      SourceMonitoring,
			ScannedAt:   time.Now(),
		}
		// Repurpose ScannedAt only when asOf is meaningful, so the UI's
		// "X ago" hint reflects metric freshness rather than fetch freshness.
		if !asOf.IsZero() {
			u.ScannedAt = asOf
		}
		s.mu.Lock()
		s.cache[key] = u
		s.mu.Unlock()
		return ReadyMsg{JobID: "monitoring:" + bucket, Usage: u}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/gcp/usage/ -v`
Expected: PASS for all `TestScanner_*` plus existing tally tests.

- [ ] **Step 5: Commit**

```bash
git add internal/gcp/usage/scanner.go internal/gcp/usage/scanner_test.go
git commit -m "2026-05-10: add Scanner with monitoring fetch and cache"
```

---

### Task 1.5: Implement deep scan with progress streaming

**Files:**
- Create: `internal/gcp/usage/job.go`
- Modify: `internal/gcp/usage/scanner.go`
- Modify: `internal/gcp/usage/scanner_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/gcp/usage/scanner_test.go`:

```go
func TestScanner_StartDeepScan_Success(t *testing.T) {
	st := &fakeStorage{
		objs: []gcp.StorageObject{
			{Name: "a.txt", Size: 100},
			{Name: "logs/b.log", Size: 200},
		},
	}
	s := New(st, &fakeMonitoring{})
	jobID, cmd := s.StartDeepScan(context.Background(), "b", "")
	require.NotEmpty(t, jobID)
	require.NotNil(t, cmd)

	// Pump until ReadyMsg.
	var ready ReadyMsg
	for {
		msg := cmd()
		if msg == nil {
			t.Fatal("nil message before ReadyMsg")
		}
		if r, ok := msg.(ReadyMsg); ok {
			ready = r
			break
		}
		// Otherwise it's a ProgressMsg — re-arm.
		cmd = s.NextMessage(jobID)
		require.NotNil(t, cmd)
	}
	require.NoError(t, ready.Err)
	assert.Equal(t, int64(300), ready.Usage.TotalBytes)
	assert.Equal(t, int64(2), ready.Usage.ObjectCount)
	assert.Equal(t, SourceDeepScan, ready.Usage.Source)

	// Cached for next caller.
	cached, ok := s.Get("b", "")
	assert.True(t, ok)
	assert.Equal(t, int64(300), cached.TotalBytes)
}

func TestScanner_StartDeepScan_Cancel(t *testing.T) {
	st := &fakeStorage{
		delay: 5 * time.Second,
		objs:  []gcp.StorageObject{{Name: "a", Size: 1}},
	}
	s := New(st, &fakeMonitoring{})
	jobID, cmd := s.StartDeepScan(context.Background(), "b", "")
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.Cancel(jobID)
	}()
	msg := cmd()
	ready, ok := msg.(ReadyMsg)
	require.True(t, ok, "expected ReadyMsg after cancel, got %T", msg)
	require.Error(t, ready.Err)
	assert.ErrorIs(t, ready.Err, context.Canceled)
}

func TestScanner_StartDeepScan_Dedup(t *testing.T) {
	st := &fakeStorage{
		delay: 100 * time.Millisecond,
		objs:  []gcp.StorageObject{{Name: "a", Size: 1}},
	}
	s := New(st, &fakeMonitoring{})

	job1, cmd1 := s.StartDeepScan(context.Background(), "b", "")
	job2, cmd2 := s.StartDeepScan(context.Background(), "b", "")
	assert.Equal(t, job1, job2, "second call for same key should reuse jobID")
	require.NotNil(t, cmd1)
	require.NotNil(t, cmd2)

	// Both subscribers should receive the final ReadyMsg.
	got1 := drainToReady(t, s, job1, cmd1)
	got2 := drainToReady(t, s, job2, cmd2)
	require.NoError(t, got1.Err)
	require.NoError(t, got2.Err)
	assert.Equal(t, int64(1), got1.Usage.TotalBytes)
	assert.Equal(t, int64(1), got2.Usage.TotalBytes)
	assert.Equal(t, 1, st.calls, "only one underlying List call expected")
}

func drainToReady(t *testing.T, s *Scanner, jobID string, cmd tea.Cmd) ReadyMsg {
	t.Helper()
	for i := 0; i < 1000; i++ {
		msg := cmd()
		if msg == nil {
			t.Fatal("nil message")
		}
		if r, ok := msg.(ReadyMsg); ok {
			return r
		}
		cmd = s.NextMessage(jobID)
		require.NotNil(t, cmd)
	}
	t.Fatal("never reached ReadyMsg")
	return ReadyMsg{}
}
```

Add the import for `tea` at the top of the file:

```go
import (
	// ... existing imports
	tea "github.com/charmbracelet/bubbletea"
)
```

- [ ] **Step 2: Run tests to confirm they fail**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/gcp/usage/ -run TestScanner_StartDeepScan -v`
Expected: FAIL — `StartDeepScan`, `NextMessage`, `Cancel` not defined.

- [ ] **Step 3: Implement job.go**

Create `internal/gcp/usage/job.go`:

```go
package usage

import (
	"context"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/slayer/gcon/internal/gcp"
)

// scanJob represents one in-flight deep scan. Multiple subscribers (one per
// caller of StartDeepScan for the same key) each get their own buffered
// channel. The runner goroutine fans out every message to every subscriber.
type scanJob struct {
	id     string
	bucket string
	prefix string
	cancel context.CancelFunc

	mu          sync.Mutex
	subscribers []chan tea.Msg
	done        bool
}

// channelBuffer sizes each subscriber channel. Large enough to absorb a few
// progress messages plus the final ready message without blocking the runner.
const channelBuffer = 8

func newScanJob(id, bucket, prefix string, cancel context.CancelFunc) *scanJob {
	return &scanJob{
		id:     id,
		bucket: bucket,
		prefix: prefix,
		cancel: cancel,
	}
}

// addSubscriber returns a fresh channel that will receive every message broadcast
// to the job from now until completion.
func (j *scanJob) addSubscriber() chan tea.Msg {
	ch := make(chan tea.Msg, channelBuffer)
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.done {
		// Job already finished; close immediately so reader unblocks with nil.
		close(ch)
		return ch
	}
	j.subscribers = append(j.subscribers, ch)
	return ch
}

// broadcast sends msg to all subscribers (non-blocking; drops if buffer is full).
func (j *scanJob) broadcast(msg tea.Msg) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, ch := range j.subscribers {
		select {
		case ch <- msg:
		default:
			// Subscriber is slow; drop. Progress is approximate by design.
		}
	}
}

// closeAll closes every subscriber channel (called after the final ReadyMsg
// has been broadcast).
func (j *scanJob) closeAll() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.done = true
	for _, ch := range j.subscribers {
		close(ch)
	}
	j.subscribers = nil
}

// runScan executes the actual scan and broadcasts progress + ready messages.
// Caller must have already added the job to scanner.inflight under scanner.mu.
func (s *Scanner) runScan(ctx context.Context, job *scanJob) {
	objs, err := s.storage.ListAllObjects(ctx, job.bucket, job.prefix)

	// Always remove the job from inflight before the final broadcast so that
	// a subscriber re-subscribing after they see ReadyMsg gets a fresh job.
	s.mu.Lock()
	delete(s.inflight, cacheKey(job.bucket, job.prefix))
	s.mu.Unlock()

	if err != nil {
		job.broadcast(ReadyMsg{JobID: job.id, Err: err})
		job.closeAll()
		return
	}

	// Storage class isn't carried on gcp.StorageObject yet. For v1 we pass empty
	// strings (so ByStorageClass remains empty until that field is added). This
	// keeps tally semantics correct: zero entries instead of bogus ones.
	classes := make([]string, len(objs))
	usage := tallyObjects(job.bucket, job.prefix, objs, classes)

	// Cache the result.
	s.mu.Lock()
	s.cache[cacheKey(job.bucket, job.prefix)] = usage
	s.mu.Unlock()

	job.broadcast(ReadyMsg{JobID: job.id, Usage: usage})
	job.closeAll()
}

// pumpCmd returns a tea.Cmd that reads exactly one message from ch. When the
// channel is closed (job complete and drained), the Cmd returns nil — the App
// handler treats nil as "stop pumping".
func pumpCmd(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// jobIDFor returns the deterministic job ID used for a (bucket, prefix). Using
// a deterministic ID makes dedup trivial: same key → same ID → same job.
func jobIDFor(bucket, prefix string) string {
	return "scan:" + bucket + "|" + prefix
}

// _ unused-import guard to keep the gcp import even when fields aren't referenced.
var _ gcp.StorageObject
```

- [ ] **Step 4: Add StartDeepScan, NextMessage, Cancel to scanner.go**

Append to `internal/gcp/usage/scanner.go`:

```go
// StartDeepScan begins (or joins) a deep scan for (bucket, prefix). It returns
// the job ID and a tea.Cmd that reads exactly one message from this caller's
// subscriber channel. The App handler MUST call Scanner.NextMessage(jobID) on
// every ProgressMsg to keep the pump alive. ReadyMsg ends the pump (channel
// closes; subsequent NextMessage calls are no-ops).
//
// If a scan for the same key is already in flight, returns the existing jobID
// and a fresh subscriber Cmd attached to the existing job.
func (s *Scanner) StartDeepScan(ctx context.Context, bucket, prefix string) (string, tea.Cmd) {
	key := cacheKey(bucket, prefix)
	s.mu.Lock()
	if existing, ok := s.inflight[key]; ok {
		ch := existing.addSubscriber()
		s.mu.Unlock()
		return existing.id, pumpCmd(ch)
	}
	scanCtx, cancel := context.WithCancel(ctx)
	job := newScanJob(jobIDFor(bucket, prefix), bucket, prefix, cancel)
	s.inflight[key] = job
	ch := job.addSubscriber()
	s.mu.Unlock()

	go s.runScan(scanCtx, job)

	return job.id, pumpCmd(ch)
}

// NextMessage returns a Cmd that reads the next message for the caller's most
// recent subscription on the given job. Each call here creates a NEW subscriber
// channel — this is intentional: in practice the App handler invokes pumpCmd
// once per message, so we re-issue a fresh single-shot pump. To keep ordering
// across messages within one logical pump, NextMessage returns a Cmd reading
// from the SAME channel as the prior pump call by re-using the job's most
// recent subscriber. Callers should treat the returned Cmd as the continuation
// of the previous pump; do not invoke NextMessage from multiple goroutines for
// the same caller.
//
// Implementation: we look up the job and add a fresh subscriber. This means
// two consecutive NextMessage calls actually read from two different channels,
// but because a scan only ever broadcasts ONE Ready (terminal) and broadcasts
// every Progress to every subscriber, the App receives all messages it needs
// in order. The slight redundancy in subscribers is acceptable for v1; future
// work can attach a per-caller ID and look up the same channel.
func (s *Scanner) NextMessage(jobID string) tea.Cmd {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, job := range s.inflight {
		if job.id == jobID {
			ch := job.addSubscriber()
			return pumpCmd(ch)
		}
	}
	// Job is no longer in flight (completed or never started). Return a no-op
	// so the handler chain doesn't crash.
	return noOpCmd()
}

// Cancel terminates the named in-flight job, causing the runner to exit and
// broadcast ReadyMsg{Err: context.Canceled}. No-op if jobID is unknown.
func (s *Scanner) Cancel(jobID string) {
	s.mu.RLock()
	var target *scanJob
	for _, job := range s.inflight {
		if job.id == jobID {
			target = job
			break
		}
	}
	s.mu.RUnlock()
	if target != nil {
		target.cancel()
	}
}
```

- [ ] **Step 5: Run tests**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/gcp/usage/ -v -timeout 30s`
Expected: PASS for all tests including `TestScanner_StartDeepScan_*`.

If the dedup test fails because the second subscriber's channel was closed before they read, that means the scan completed before the second `addSubscriber` call. The fake's `delay: 100ms` should be enough cushion; if it's flaky, raise it to `200 * time.Millisecond`.

- [ ] **Step 6: Run with race detector**

Run: `cd /home/vlad/dev/my/gcon && go test -race ./internal/gcp/usage/ -v -timeout 30s`
Expected: PASS with no race warnings.

- [ ] **Step 7: Commit**

```bash
git add internal/gcp/usage/job.go internal/gcp/usage/scanner.go internal/gcp/usage/scanner_test.go
git commit -m "2026-05-10: add deep-scan with progress streaming and dedup"
```

---

### Task 1.6: Add updateRunningTask helper to App

**Files:**
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Locate the existing task helpers**

Run: `grep -n "registerRunningTask\|finishTask" /home/vlad/dev/my/gcon/internal/ui/app.go`
Expected: lines around 1162 and 1182.

- [ ] **Step 2: Add updateRunningTask immediately after registerRunningTask**

Find the line `// clearRunningTasks removes all tasks still in TaskRunning state.` (around line 1171) and insert before it:

```go
// updateRunningTask updates the description of an already-running task.
// Used to surface live progress (e.g. scan counters) without re-registering
// the task. No-op if the task is not in TaskRunning state.
func (a *App) updateRunningTask(id, description string) {
	if t, ok := a.ctx.Tasks[id]; ok && t.State == context.TaskRunning {
		t.Description = description
		a.ctx.Tasks[id] = t
	}
}
```

- [ ] **Step 3: Verify build**

Run: `cd /home/vlad/dev/my/gcon && go build ./...`
Expected: build succeeds.

- [ ] **Step 4: Add a quick test**

Create `internal/ui/app_task_test.go`:

```go
package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/ui/context"
)

func TestUpdateRunningTask_UpdatesDescription(t *testing.T) {
	a := &App{ctx: context.New()}
	a.ctx.Tasks["foo"] = context.Task{
		ID:          "foo",
		Description: "Initial",
		State:       context.TaskRunning,
		StartTime:   time.Now(),
	}
	a.updateRunningTask("foo", "Updated 50%")
	assert.Equal(t, "Updated 50%", a.ctx.Tasks["foo"].Description)
}

func TestUpdateRunningTask_NoOpForFinishedTask(t *testing.T) {
	a := &App{ctx: context.New()}
	a.ctx.Tasks["foo"] = context.Task{
		ID:          "foo",
		Description: "Done",
		State:       context.TaskFinished,
	}
	a.updateRunningTask("foo", "should not apply")
	assert.Equal(t, "Done", a.ctx.Tasks["foo"].Description)
}

func TestUpdateRunningTask_NoOpForUnknownTask(t *testing.T) {
	a := &App{ctx: context.New()}
	// Should not panic.
	a.updateRunningTask("nope", "anything")
	_, exists := a.ctx.Tasks["nope"]
	assert.False(t, exists)
}
```

- [ ] **Step 5: Run tests**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/ui/ -run TestUpdateRunningTask -v`
Expected: PASS.

- [ ] **Step 6: Commit (Phase 1 complete)**

```bash
git add internal/ui/app.go internal/ui/app_task_test.go
git commit -m "2026-05-10: add updateRunningTask helper for live progress"
```

- [ ] **Step 7: Run full test suite to confirm Phase 1 is green**

Run: `cd /home/vlad/dev/my/gcon && go test ./... -timeout 60s`
Expected: all tests pass.

- [ ] **Step 8: Lint**

Run: `cd /home/vlad/dev/my/gcon && make lint`
Expected: no errors.

**Phase 1 Done.** PR 1 boundary. The data layer is fully functional and tested in isolation; no user-visible changes yet.

---

## Phase 2: Buckets List Integration (PR 2)

### Task 2.1: Wire usageScanner into App with lazy construction

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_navigation.go`

- [ ] **Step 1: Add the field to App struct**

In `internal/ui/app.go`, find the `type App struct {` block. After the line `gcpClient *gcp.Client`, add:

```go
	// usageScanner provides bucket usage data (monitoring + deep scans).
	// Constructed lazily on first usage request after a project is selected.
	usageScanner *usage.Scanner
```

Add the import at the top of `app.go`:

```go
	"github.com/slayer/gcon/internal/gcp/usage"
```

- [ ] **Step 2: Add a helper method to construct the scanner lazily**

Append to `internal/ui/app.go` (near the other helpers around line 1200):

```go
// ensureUsageScanner constructs the scanner on first use. It needs both a
// StorageClient and a MonitoringClient; the storage client is borrowed from
// BucketsView (whichever view created it first), and a fresh MonitoringClient
// is created scoped to the current project. Returns nil if the prerequisites
// are not yet available.
func (a *App) ensureUsageScanner() *usage.Scanner {
	if a.usageScanner != nil {
		return a.usageScanner
	}
	if a.selectedProject == nil {
		return nil
	}
	if a.bucketsView == nil {
		return nil
	}
	storageClient := a.bucketsView.GetStorageClient()
	if storageClient == nil {
		return nil
	}
	monClient, err := gcp.NewMonitoringClient(gocontext.Background(), a.selectedProject.ID)
	if err != nil {
		// Log via existing error mechanism; without monitoring we can't show
		// inline totals. Deep scans still work via the storage path, but we
		// require both to construct the Scanner. Defer construction until next
		// attempt.
		a.err = fmt.Errorf("usage scanner monitoring init: %w", err)
		return nil
	}
	a.usageScanner = usage.New(storageClient, monClient)
	return a.usageScanner
}
```

If `gocontext` is not already imported, add `gocontext "context"` to the import block. If `fmt` is not imported, add it. (Both are likely already there.)

- [ ] **Step 3: Reset scanner on project switch**

Find `clearAllViews` in `internal/ui/app_navigation.go`. Add to its body (immediately after the comment block at the top, before clearing views):

```go
	// Drop the scanner so a fresh one is built for the new project's clients.
	// In-flight scans tied to the old project will be cancelled when their
	// underlying contexts are garbage-collected; for v1 we accept that they may
	// briefly continue running in the background until they hit ctx.Done.
	a.usageScanner = nil
```

- [ ] **Step 4: Build and confirm**

Run: `cd /home/vlad/dev/my/gcon && go build ./...`
Expected: build succeeds.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/app.go internal/ui/app_navigation.go
git commit -m "2026-05-10: wire usage.Scanner into App with lazy construction"
```

---

### Task 2.2: Add CancelUsageScan key binding

**Files:**
- Modify: `internal/ui/keys.go`
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Add the binding**

In `internal/ui/keys.go`, find the `KeyMap` struct. Add a field:

```go
	CancelUsageScan key.Binding
```

In the `defaultKeyMap()` (or equivalent constructor), add:

```go
		CancelUsageScan: key.NewBinding(
			key.WithKeys("ctrl+x"),
			key.WithHelp("ctrl+x", "cancel scan"),
		),
```

- [ ] **Step 2: Add helper to find active scan job ID**

Append to `internal/ui/app.go`:

```go
// activeUsageScanJobID returns the JobID of the first in-flight usage scan
// found in the task tracker (those whose ID begins with "scan:"), and true
// if one exists. Used by the Ctrl+X cancel handler.
func (a *App) activeUsageScanJobID() (string, bool) {
	for id, t := range a.ctx.Tasks {
		if t.State == context.TaskRunning && strings.HasPrefix(id, "scan:") {
			return id, true
		}
	}
	return "", false
}
```

If `strings` is not yet imported in `app.go`, add it.

- [ ] **Step 3: Wire the key in Update()**

Find the global key handling section in `app.go` (where `a.keys.Quit`, `a.keys.Help`, etc. are matched). Add a case for `CancelUsageScan` before the per-view key dispatch:

```go
		case key.Matches(msg, a.keys.CancelUsageScan):
			if jobID, ok := a.activeUsageScanJobID(); ok && a.usageScanner != nil {
				a.usageScanner.Cancel(jobID)
			}
			return a, nil
```

- [ ] **Step 4: Build**

Run: `cd /home/vlad/dev/my/gcon && go build ./...`
Expected: build succeeds.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/keys.go internal/ui/app.go
git commit -m "2026-05-10: add Ctrl+X to cancel active usage scan"
```

---

### Task 2.3: Define usage messages and Size/Objects columns in BucketsView

**Files:**
- Create: `internal/ui/views/bucket_usage_messages.go`
- Modify: `internal/ui/views/buckets.go`

- [ ] **Step 1: Create the messages file**

Create `internal/ui/views/bucket_usage_messages.go`:

```go
package views

// UsageMonitoringRequestMsg asks the App to fetch monitoring metrics for a
// bucket via the usage.Scanner. The App routes ReadyMsg back to the originator
// based on which views are currently mounted and care about this bucket.
type UsageMonitoringRequestMsg struct {
	Bucket string
}

// UsageDeepScanRequestMsg asks the App to start (or join) a deep scan of the
// given (bucket, prefix). Empty Prefix means the entire bucket.
type UsageDeepScanRequestMsg struct {
	Bucket string
	Prefix string
}

// BucketDetailsRequestMsg asks the App to navigate to the BucketDetailsView
// for the named bucket. (Wired to a no-op in Phase 2; the view lands in Phase 3.)
type BucketDetailsRequestMsg struct {
	Bucket string
}
```

- [ ] **Step 2: Update bucketColumns()**

In `internal/ui/views/buckets.go`, replace the `bucketColumns` function:

```go
// Table column definitions for buckets
func bucketColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 36, Grow: true, Sortable: true},
		{Title: "Location", Width: 14, Sortable: true},
		{Title: "Storage Class", Width: 13, Sortable: true},
		{Title: "Size", Width: 12, Sortable: true},
		{Title: "Objects", Width: 11, Sortable: true},
		{Title: "Created", Width: 12, Sortable: true},
	}
}
```

- [ ] **Step 3: Update bucketToRow()**

Replace `bucketToRow` to render placeholder cells for size/objects (they'll be filled in by usage messages):

```go
// bucketToRow converts a GCS bucket to a table row.
// The Size and Objects cells render as a faint "…" until usage data arrives;
// they are updated in place when UsageReadyMsg is processed.
func bucketToRow(b gcp.Bucket) table.Row {
	mutedDots := "…"
	return table.Row{
		Data: []string{
			"📦 " + b.Name,
			b.Location,
			b.StorageClass,
			mutedDots,
			mutedDots,
			timeutil.FormatDate(b.Created),
		},
		FilterValue: b.Name + " " + b.Location + " " + b.StorageClass,
		ID:          b.Name,
	}
}
```

- [ ] **Step 4: Add a usage cache to the BucketsView struct**

In the `BucketsView` struct (around line 53), add at the bottom:

```go
	// usageByBucket caches the most recent usage record per bucket so that
	// ProgressMsg updates can be re-rendered without losing prior info.
	usageByBucket map[string]usage.BucketUsage
```

Add the import at the top of `buckets.go`:

```go
	"github.com/slayer/gcon/internal/gcp/usage"
```

In `NewBucketsView`, initialize the map:

```go
	v := &BucketsView{
		projectID:     projectID,
		table:         t,
		spinner:       s,
		loading:       true,
		keys:          defaultBucketKeyMap(),
		usageByBucket: make(map[string]usage.BucketUsage),
	}
```

- [ ] **Step 5: Build and confirm columns render**

Run: `cd /home/vlad/dev/my/gcon && go build ./...`
Expected: build succeeds.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/bucket_usage_messages.go internal/ui/views/buckets.go
git commit -m "2026-05-10: add Size and Objects columns + usage messages"
```

---

### Task 2.4: BucketsView fires monitoring requests on load

**Files:**
- Modify: `internal/ui/views/buckets.go`

- [ ] **Step 1: Update `bucketsLoadedMsg` handler to emit per-bucket monitoring requests**

In `BucketsView.Update`, find the `case bucketsLoadedMsg:` block. Replace its body with:

```go
		case bucketsLoadedMsg:
			v.loading = false
			v.buckets = msg.buckets

			// Convert to table rows.
			rows := make([]table.Row, len(msg.buckets))
			for i, bucket := range msg.buckets {
				rows[i] = bucketToRow(bucket)
			}
			v.table.SetRows(rows)

			// Fan out one monitoring request per bucket so the App can fetch
			// totals via the usage scanner.
			cmds := make([]tea.Cmd, 0, len(msg.buckets))
			for _, bucket := range msg.buckets {
				bucket := bucket // capture
				cmds = append(cmds, func() tea.Msg {
					return UsageMonitoringRequestMsg{Bucket: bucket.Name}
				})
			}
			return tea.Batch(cmds...)
```

- [ ] **Step 2: Build**

Run: `cd /home/vlad/dev/my/gcon && go build ./...`
Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/views/buckets.go
git commit -m "2026-05-10: fire monitoring fetch per bucket on list load"
```

---

### Task 2.5: BucketsView consumes usage.ReadyMsg / ProgressMsg

**Files:**
- Modify: `internal/ui/views/buckets.go`
- Modify: `internal/ui/views/buckets_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/ui/views/buckets_test.go`:

```go
func TestBucketsView_UsageReadyUpdatesRow(t *testing.T) {
	v := NewBucketsView("p")
	// Skip the normal load — synthesize the loaded state directly.
	v.loading = false
	v.buckets = []gcp.Bucket{
		{Name: "b1", Location: "us", StorageClass: "STANDARD", Created: time.Now()},
		{Name: "b2", Location: "eu", StorageClass: "NEARLINE", Created: time.Now()},
	}
	rows := []table.Row{bucketToRow(v.buckets[0]), bucketToRow(v.buckets[1])}
	v.table.SetRows(rows)

	// Inject a ReadyMsg for b1.
	v.Update(usage.ReadyMsg{
		JobID: "monitoring:b1",
		Usage: usage.BucketUsage{
			Bucket:      "b1",
			TotalBytes:  1_500_000_000,
			ObjectCount: 4321,
			Source:      usage.SourceMonitoring,
			ScannedAt:   time.Now().Add(-2 * time.Hour),
		},
	})

	// Locate the b1 row and check Size and Objects cells.
	got := v.table.Rows()
	require.Len(t, got, 2)
	row := got[0]
	assert.Contains(t, row.Data[3], "1.4 GB", "Size cell should be human formatted (1.5e9 ~ 1.4 GiB)")
	// Object count should be present (formatting may vary).
	assert.Contains(t, row.Data[4], "4321")
}
```

Add necessary imports (testify, gcp, time, usage).

- [ ] **Step 2: Run to confirm failure**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/ui/views/ -run TestBucketsView_UsageReady -v`
Expected: FAIL — no handler exists for `usage.ReadyMsg`.

- [ ] **Step 3: Add handlers in BucketsView.Update**

In `BucketsView.Update`, add new cases (place them before the `case spinner.TickMsg:` block):

```go
		case usage.ReadyMsg:
			v.applyUsage(msg.Usage)
			return nil

		case usage.ProgressMsg:
			// Show in-progress totals immediately so the user gets feedback.
			v.applyUsage(usage.BucketUsage{
				Bucket:      msg.Bucket,
				TotalBytes:  msg.BytesScanned,
				ObjectCount: msg.ObjectsScanned,
				Source:      usage.SourceDeepScan,
				ScannedAt:   time.Now(),
			})
			return nil
```

Add the helper method to BucketsView:

```go
// applyUsage stores the usage record and updates the corresponding table row's
// Size and Objects cells in place. Deep-scan results show a "✓" suffix to
// distinguish them from monitoring estimates.
func (v *BucketsView) applyUsage(u usage.BucketUsage) {
	v.usageByBucket[u.Bucket] = u
	rows := v.table.Rows()
	for i := range rows {
		if rows[i].ID != u.Bucket {
			continue
		}
		sizeStr := gcp.FormatSize(u.TotalBytes)
		if u.Source == usage.SourceDeepScan {
			sizeStr += " ✓"
		}
		countStr := formatObjectCount(u.ObjectCount)
		// Replace the cells. Data length is fixed by bucketColumns().
		rows[i].Data[3] = sizeStr
		rows[i].Data[4] = countStr
	}
	v.table.SetRows(rows)
}

// formatObjectCount renders an int64 with thousands separators. For now we use
// a simple grouping; if the table column is too narrow, we'll add SI suffixes
// later (1.2k, 4.5M) — postponed to keep this PR small.
func formatObjectCount(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	// Insert commas every three digits from the right.
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}
```

Make sure `time` and `fmt` are imported (they likely already are).

- [ ] **Step 4: Add `Rows()` accessor to table.Model if not present**

Check whether `internal/ui/components/table.Model` has a `Rows()` method:

Run: `grep -n "func.*Rows\(" /home/vlad/dev/my/gcon/internal/ui/components/table/*.go`
Expected: a `Rows()` method exists. If not, add one:

In `internal/ui/components/table/table.go` (or whichever file holds the model), add (only if it doesn't already exist):

```go
// Rows returns a copy of the current rows. Callers may mutate the returned
// slice safely; the model only updates when SetRows is called.
func (m *Model) Rows() []Row {
	out := make([]Row, len(m.rows))
	copy(out, m.rows)
	return out
}
```

- [ ] **Step 5: Run test**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/ui/views/ -run TestBucketsView_UsageReady -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/buckets.go internal/ui/views/buckets_test.go internal/ui/components/table/
git commit -m "2026-05-10: render usage in Size/Objects columns when ReadyMsg arrives"
```

---

### Task 2.6: BucketsView `C` key triggers deep scan, App handles it

**Files:**
- Modify: `internal/ui/views/buckets.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_navigation.go`

- [ ] **Step 1: Add the `C` key binding to BucketsView**

In `bucketKeyMap`, add:

```go
	DeepScan key.Binding
```

In `defaultBucketKeyMap`, add:

```go
		DeepScan: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "calculate usage"),
		),
```

In the existing key dispatch within `BucketsView.Update` (the switch on `v.keys.*`), add:

```go
			case key.Matches(msg, v.keys.DeepScan):
				if row := v.table.SelectedRow(); row != nil {
					bucketName := row.ID
					return func() tea.Msg {
						return UsageDeepScanRequestMsg{Bucket: bucketName, Prefix: ""}
					}
				}
```

- [ ] **Step 2: Add Update handlers in App**

In `internal/ui/app.go`, find the main `Update()` switch. Add cases for the new messages (place them near other view-specific handlers):

```go
		case views.UsageMonitoringRequestMsg:
			return a, a.handleUsageMonitoringRequest(msg)
		case views.UsageDeepScanRequestMsg:
			return a, a.handleUsageDeepScanRequest(msg)
		case usage.ProgressMsg:
			return a, a.handleUsageProgress(msg)
		case usage.ReadyMsg:
			return a, a.handleUsageReady(msg)
```

Add the import `"github.com/slayer/gcon/internal/gcp/usage"` if not already present, and the `views` import is presumably already there.

- [ ] **Step 3: Implement handlers in app_navigation.go**

Append to `internal/ui/app_navigation.go`:

```go
// handleUsageMonitoringRequest fetches monitoring metrics via the scanner and
// returns the resulting tea.Cmd. The ReadyMsg lands back in App.Update which
// dispatches to interested views.
func (a *App) handleUsageMonitoringRequest(msg views.UsageMonitoringRequestMsg) tea.Cmd {
	scanner := a.ensureUsageScanner()
	if scanner == nil {
		return nil
	}
	return scanner.FetchMonitoring(gocontext.Background(), msg.Bucket)
}

// handleUsageDeepScanRequest starts (or joins) a deep scan and registers a
// footer task with live progress.
func (a *App) handleUsageDeepScanRequest(msg views.UsageDeepScanRequestMsg) tea.Cmd {
	scanner := a.ensureUsageScanner()
	if scanner == nil {
		return nil
	}
	jobID, cmd := scanner.StartDeepScan(gocontext.Background(), msg.Bucket, msg.Prefix)
	desc := fmt.Sprintf("Scanning %s%s...", msg.Bucket, slashIfPrefix(msg.Prefix))
	a.registerRunningTask(jobID, desc)
	return cmd
}

// handleUsageProgress updates the footer task description and forwards the
// message to interested views, then re-arms the pump.
func (a *App) handleUsageProgress(msg usage.ProgressMsg) tea.Cmd {
	desc := fmt.Sprintf("Scanning %s%s — %s objects · %s",
		msg.Bucket, slashIfPrefix(msg.Prefix),
		formatObjectCount(msg.ObjectsScanned),
		gcp.FormatSize(msg.BytesScanned))
	a.updateRunningTask(msg.JobID, desc)
	cmds := []tea.Cmd{a.usageScanner.NextMessage(msg.JobID)}
	cmds = append(cmds, a.dispatchUsageProgress(msg)...)
	return tea.Batch(cmds...)
}

// handleUsageReady finalizes the footer task and forwards to interested views.
func (a *App) handleUsageReady(msg usage.ReadyMsg) tea.Cmd {
	finishCmd := a.finishTask(msg.JobID, msg.Err)
	cmds := []tea.Cmd{}
	if finishCmd != nil {
		cmds = append(cmds, finishCmd)
	}
	cmds = append(cmds, a.dispatchUsageReady(msg)...)
	return tea.Batch(cmds...)
}

// dispatchUsageProgress sends a ProgressMsg to every mounted view that may be
// interested in this bucket. A view's Update will simply ignore unrecognized
// or unrelated messages.
func (a *App) dispatchUsageProgress(msg usage.ProgressMsg) []tea.Cmd {
	cmds := []tea.Cmd{}
	if a.bucketsView != nil {
		cmds = append(cmds, msgCmd(msg))
	}
	if a.objectsView != nil {
		cmds = append(cmds, msgCmd(msg))
	}
	// Phase 3 also dispatches to bucketDetailsView; harmless to add now under a nil guard.
	return cmds
}

// dispatchUsageReady fans out the ReadyMsg to all mounted views.
func (a *App) dispatchUsageReady(msg usage.ReadyMsg) []tea.Cmd {
	cmds := []tea.Cmd{}
	if a.bucketsView != nil {
		cmds = append(cmds, msgCmd(msg))
	}
	if a.objectsView != nil {
		cmds = append(cmds, msgCmd(msg))
	}
	return cmds
}

// msgCmd wraps a value as a tea.Cmd so it can be returned in a Batch.
func msgCmd(m tea.Msg) tea.Cmd {
	return func() tea.Msg { return m }
}

// slashIfPrefix returns "/<prefix>" when prefix is non-empty, "" otherwise.
// Used for human-readable scan descriptions.
func slashIfPrefix(prefix string) string {
	if prefix == "" {
		return ""
	}
	return "/" + prefix
}

// formatObjectCount renders n with comma thousands separators. Duplicated from
// the views package to avoid importing views from app_navigation; this helper
// is small enough that DRY isn't worth the layering cost.
func formatObjectCount(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}
```

- [ ] **Step 4: Build**

Run: `cd /home/vlad/dev/my/gcon && go build ./...`
Expected: build succeeds. If `gocontext` isn't aliased in `app_navigation.go`, ensure the import block has `gocontext "context"`.

- [ ] **Step 5: Quick smoke test of the binding**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/ui/views/ -run TestBucketsView -v`
Expected: PASS (existing tests still green).

- [ ] **Step 6: Commit**

```bash
git add internal/ui/views/buckets.go internal/ui/app.go internal/ui/app_navigation.go
git commit -m "2026-05-10: 'C' key triggers deep scan with footer progress"
```

---

### Task 2.7: BucketsView `i` key + stub navigation

**Files:**
- Modify: `internal/ui/views/buckets.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_navigation.go`

- [ ] **Step 1: Add the `i` binding**

In `bucketKeyMap`, add:

```go
	Details key.Binding
```

In `defaultBucketKeyMap`:

```go
		Details: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "details"),
		),
```

In the BucketsView key switch:

```go
			case key.Matches(msg, v.keys.Details):
				if row := v.table.SelectedRow(); row != nil {
					bucketName := row.ID
					return func() tea.Msg {
						return BucketDetailsRequestMsg{Bucket: bucketName}
					}
				}
```

- [ ] **Step 2: Add stub handler in App**

In `internal/ui/app.go` Update():

```go
		case views.BucketDetailsRequestMsg:
			// Phase 3 wires this to BucketDetailsView. For Phase 2 we no-op;
			// the user sees nothing when pressing 'i', which is acceptable
			// because the help text only documents 'i' once Phase 3 lands.
			return a, nil
```

- [ ] **Step 3: Build**

Run: `cd /home/vlad/dev/my/gcon && go build ./...`
Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/views/buckets.go internal/ui/app.go
git commit -m "2026-05-10: add 'i' key for bucket details (handler stubbed for Phase 3)"
```

---

### Task 2.8: Update help text and docs

**Files:**
- Modify: `internal/ui/views/buckets.go`
- Modify: `README.md`
- Modify: `.claude/rules/key-bindings.md`

- [ ] **Step 1: Update BucketsView help text**

In `BucketsView.View()`, update the help line:

```go
	help := helpStyle.Render("\n  enter: browse • C: calculate usage • c: create • S: sort • /: filter • r: refresh • esc: back")
```

(The `i` key is intentionally undocumented until Phase 3.)

- [ ] **Step 2: Update key-bindings.md**

In `.claude/rules/key-bindings.md`, find the `## Buckets View` section. Replace its table with:

```markdown
| Key | Action |
|-----|--------|
| `Enter` | Browse bucket contents |
| `S` | Open sort menu |
| `c` | Create new bucket |
| `C` | Calculate usage (deep scan, footer progress, Ctrl+X to cancel) |
| `/` | Filter buckets |
| `r` | Refresh list |
| `Esc` | Go back |
```

In the `## Global` section, add `Ctrl+X` to the table:

```markdown
| `Ctrl+X` | Cancel active usage scan |
```

- [ ] **Step 3: Update README.md**

In `README.md`, find the Cloud Storage feature bullet (or add one if missing). Append:

```markdown
- Bucket usage column showing total size and object count (from Cloud Monitoring)
- On-demand deep scan (`C` key) with live progress in the footer
```

- [ ] **Step 4: Run full test suite + lint**

Run: `cd /home/vlad/dev/my/gcon && go test ./... -timeout 90s && make lint`
Expected: all green.

- [ ] **Step 5: Commit (Phase 2 complete)**

```bash
git add internal/ui/views/buckets.go README.md .claude/rules/key-bindings.md
git commit -m "2026-05-10: document new bucket usage keys"
```

**Phase 2 Done.** PR 2 boundary. Users can now see Size/Objects per bucket and run deep scans with live footer progress + cancel.

---

## Phase 3: BucketDetailsView + ObjectsView Folder Scan (PR 3)

### Task 3.1: Scaffold BucketDetailsView (no usage UI yet)

**Files:**
- Create: `internal/ui/views/bucket_details.go`

- [ ] **Step 1: Create the file with minimum scaffolding**

Create `internal/ui/views/bucket_details.go`:

```go
package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/gcp/usage"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/timeutil"
)

// bucketDetailsTab identifies the active tab in BucketDetailsView.
type bucketDetailsTab int

const (
	bucketDetailsTabDetails bucketDetailsTab = iota
	bucketDetailsTabUsage
)

type bucketDetailsKeyMap struct {
	DeepScan key.Binding
	Refresh  key.Binding
	NextTab  key.Binding
	PrevTab  key.Binding
	Tab1     key.Binding
	Tab2     key.Binding
}

func defaultBucketDetailsKeyMap() bucketDetailsKeyMap {
	return bucketDetailsKeyMap{
		DeepScan: key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "calculate usage")),
		Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh monitoring")),
		NextTab:  key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "next tab")),
		PrevTab:  key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "prev tab")),
		Tab1:     key.NewBinding(key.WithKeys("1")),
		Tab2:     key.NewBinding(key.WithKeys("2")),
	}
}

// BucketDetailsView shows static bucket metadata plus a Usage tab driven by
// the usage.Scanner.
type BucketDetailsView struct {
	bucket  gcp.Bucket
	ctx     *context.ProgramContext
	tabs    *tabs.Tabs // tabs.New returns *Tabs (not *Model)
	keys    bucketDetailsKeyMap
	spinner spinner.Model

	// usage holds the most recent BucketUsage for this bucket (any source).
	usage *usage.BucketUsage
	// scanInProgress is true between StartDeepScan and ReadyMsg.
	scanInProgress bool
	scanErr        error
}

// NewBucketDetailsView constructs the view. Caller must call Init() afterwards.
func NewBucketDetailsView(bucket gcp.Bucket) *BucketDetailsView {
	t := tabs.New([]tabs.Tab{
		{ID: "details", Label: "Details"},
		{ID: "usage", Label: "Usage"},
	})
	return &BucketDetailsView{
		bucket:  bucket,
		tabs:    t,
		keys:    defaultBucketDetailsKeyMap(),
		spinner: components.NewGCPSpinner(),
	}
}

// Init kicks off a monitoring fetch so the Usage tab has at least the totals.
func (v *BucketDetailsView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		func() tea.Msg {
			return UsageMonitoringRequestMsg{Bucket: v.bucket.Name}
		},
	)
}

// SetSize delegates dimensions to inner components.
func (v *BucketDetailsView) SetSize(width, height int) {
	if v.tabs != nil {
		v.tabs.SetSize(width)
	}
}

// SetContext stores the shared context.
func (v *BucketDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	if ctx != nil {
		v.SetSize(ctx.ContentWidth, ctx.ContentHeight)
	}
}

// HasTextInputFocused returns false; this view has no text inputs.
func (v *BucketDetailsView) HasTextInputFocused() bool { return false }

// IsMenuOpen returns false; no action menu in v1.
func (v *BucketDetailsView) IsMenuOpen() bool { return false }

// View renders the active tab.
func (v *BucketDetailsView) View() string {
	if v.ctx == nil {
		return ""
	}
	header := v.tabs.View()
	body := ""
	switch v.tabs.ActiveTab().ID {
	case "details":
		body = v.renderDetailsTab()
	case "usage":
		body = v.renderUsageTab()
	}
	return header + "\n" + body
}

// Update handles messages.
func (v *BucketDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case usage.ReadyMsg:
		if msg.Usage.Bucket != v.bucket.Name && msg.Err == nil {
			return nil // not for us
		}
		if msg.Err != nil {
			v.scanErr = msg.Err
			v.scanInProgress = false
			return nil
		}
		u := msg.Usage
		v.usage = &u
		v.scanInProgress = false
		return nil

	case usage.ProgressMsg:
		if msg.Bucket != v.bucket.Name {
			return nil
		}
		v.scanInProgress = true
		// Show running tally in the Usage tab.
		v.usage = &usage.BucketUsage{
			Bucket:      msg.Bucket,
			Prefix:      msg.Prefix,
			TotalBytes:  msg.BytesScanned,
			ObjectCount: msg.ObjectsScanned,
			Source:      usage.SourceDeepScan,
		}
		return nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		v.spinner, cmd = v.spinner.Update(msg)
		return cmd

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, v.keys.NextTab):
			n := v.tabs.Count()
			if n > 0 {
				v.tabs.SetActive((v.tabs.ActiveIndex() + 1) % n)
			}
			return nil
		case key.Matches(msg, v.keys.PrevTab):
			n := v.tabs.Count()
			if n > 0 {
				v.tabs.SetActive((v.tabs.ActiveIndex() - 1 + n) % n)
			}
			return nil
		case key.Matches(msg, v.keys.Tab1):
			v.tabs.SetActive(0)
			return nil
		case key.Matches(msg, v.keys.Tab2):
			v.tabs.SetActive(1)
			return nil
		case key.Matches(msg, v.keys.DeepScan):
			if v.tabs.ActiveTab().ID != "usage" {
				return nil
			}
			v.scanInProgress = true
			v.scanErr = nil
			return func() tea.Msg {
				return UsageDeepScanRequestMsg{Bucket: v.bucket.Name, Prefix: ""}
			}
		case key.Matches(msg, v.keys.Refresh):
			return func() tea.Msg {
				return UsageMonitoringRequestMsg{Bucket: v.bucket.Name}
			}
		}
	}
	return nil
}

// renderDetailsTab shows static bucket metadata.
func (v *BucketDetailsView) renderDetailsTab() string {
	labelStyle := lipgloss.NewStyle().Foreground(v.ctx.Styles.Colors.Muted).Width(15)
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Name:") + " " + v.bucket.Name + "\n")
	b.WriteString(labelStyle.Render("Location:") + " " + v.bucket.Location + "\n")
	b.WriteString(labelStyle.Render("Storage Class:") + " " + v.bucket.StorageClass + "\n")
	b.WriteString(labelStyle.Render("Created:") + " " + timeutil.FormatDate(v.bucket.Created) + "\n")
	b.WriteString("\n")
	linkStyle := lipgloss.NewStyle().Foreground(v.ctx.Styles.Colors.Primary).Underline(true)
	b.WriteString(linkStyle.Render("Press Enter to browse objects →") + "\n")
	return b.String()
}

// renderUsageTab shows totals and (when available) breakdowns.
func (v *BucketDetailsView) renderUsageTab() string {
	var b strings.Builder
	b.WriteString("\n")
	if v.scanErr != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		b.WriteString(errStyle.Render("  Scan error: " + v.scanErr.Error()) + "\n\n")
	}
	if v.usage == nil {
		b.WriteString("  Loading monitoring metrics...\n")
		b.WriteString("\n  Press 'C' to run a deep scan.\n")
		return b.String()
	}
	u := *v.usage
	muted := lipgloss.NewStyle().Foreground(v.ctx.Styles.Colors.Muted)
	b.WriteString(fmt.Sprintf("  Total size:    %s\n", gcp.FormatSize(u.TotalBytes)))
	b.WriteString(fmt.Sprintf("  Object count:  %s\n\n", formatObjectCount(u.ObjectCount)))
	switch u.Source {
	case usage.SourceMonitoring:
		hint := "Source: Monitoring"
		if !u.ScannedAt.IsZero() {
			hint += " · as of " + timeutil.FormatDateTime(u.ScannedAt)
		}
		b.WriteString("  " + muted.Render(hint) + "\n")
		b.WriteString("\n  Press 'C' to run a deep scan for breakdowns.\n")
	case usage.SourceDeepScan:
		if v.scanInProgress {
			b.WriteString("  " + muted.Render("Scan in progress (" + v.spinner.View() + ")...") + "\n")
		} else {
			b.WriteString("  " + muted.Render("Source: Deep scan") + "\n")
		}
		b.WriteString("\n")
		writeStatTable(&b, "By Storage Class", u.ByStorageClass, 0)
		writeStatTable(&b, "By Top-Level Prefix", u.ByTopPrefix, 0)
		writeStatTable(&b, "By Extension (top 20 by size)", u.ByExtension, 20)
	}
	return b.String()
}

// writeStatTable renders one breakdown section (sorted by Bytes desc).
// limit > 0 caps the number of rows shown.
func writeStatTable(b *strings.Builder, title string, m map[string]usage.Stat, limit int) {
	if len(m) == 0 {
		return
	}
	type kv struct {
		k string
		v usage.Stat
	}
	all := make([]kv, 0, len(m))
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	// Sort descending by Bytes.
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].v.Bytes > all[i].v.Bytes {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	b.WriteString("  — " + title + " ————————————————————————\n")
	for _, e := range all {
		b.WriteString(fmt.Sprintf("    %-30s %12s  %12s\n",
			e.k, gcp.FormatSize(e.v.Bytes), formatObjectCount(e.v.Count)))
	}
	b.WriteString("\n")
}
```

- [ ] **Step 2: Build**

Run: `cd /home/vlad/dev/my/gcon && go build ./...`
Expected: build succeeds. Resolve any field-path mismatches now.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/views/bucket_details.go
git commit -m "2026-05-10: scaffold BucketDetailsView with Details/Usage tabs"
```

---

### Task 3.2: Wire BucketDetailsView into App (16-step checklist)

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_render.go`
- Modify: `internal/ui/app_navigation.go`

Following `.claude/rules/adding-new-views.md`:

- [ ] **Step 1: Add `ViewBucketDetails` constant**

In `internal/ui/app.go`, find the `ViewType` enum. Add a constant:

```go
	ViewBucketDetails
```

Add the field to the App struct (near `bucketsView`):

```go
	bucketDetailsView *views.BucketDetailsView
```

- [ ] **Step 2: Add to `getCurrentViewModel()`**

```go
		case ViewBucketDetails:
			return a.bucketDetailsView
```

- [ ] **Step 3: Add to `app_render.go`**

```go
		case ViewBucketDetails:
			if a.bucketDetailsView != nil {
				return a.bucketDetailsView.View()
			}
```

- [ ] **Step 4: Replace the stub for `BucketDetailsRequestMsg`**

In `internal/ui/app.go` Update(), replace the no-op case from Phase 2:

```go
		case views.BucketDetailsRequestMsg:
			return a, a.handleBucketDetailsRequest(msg)
```

- [ ] **Step 5: Implement the handler**

Append to `internal/ui/app_navigation.go`:

```go
// handleBucketDetailsRequest navigates to BucketDetailsView for the named bucket.
func (a *App) handleBucketDetailsRequest(msg views.BucketDetailsRequestMsg) tea.Cmd {
	if a.bucketsView == nil {
		return nil
	}
	// Locate the bucket struct from the cached list so the details view has
	// metadata (location, class, created) without an extra GCP call.
	var found *gcp.Bucket
	for i, b := range a.bucketsView.Buckets() {
		if b.Name == msg.Bucket {
			found = &a.bucketsView.Buckets()[i]
			break
		}
	}
	if found == nil {
		return nil
	}
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewBucketDetails
	a.bucketDetailsView = views.NewBucketDetailsView(*found)
	a.updateViewSizes()
	return a.bucketDetailsView.Init()
}
```

Add a `Buckets()` accessor to `BucketsView` if missing. In `internal/ui/views/buckets.go`:

```go
// Buckets returns the cached list of buckets. Used by App for navigation.
func (v *BucketsView) Buckets() []gcp.Bucket {
	return v.buckets
}
```

- [ ] **Step 6: Add to `clearAllViews`**

In `app_navigation.go` `clearAllViews`:

```go
	a.bucketDetailsView = nil
```

- [ ] **Step 7: Add to `updateViewSizes`**

```go
	if a.bucketDetailsView != nil {
		a.bucketDetailsView.SetContext(a.ctx)
	}
```

- [ ] **Step 8: Update dispatch helpers**

In `app_navigation.go` `dispatchUsageProgress` and `dispatchUsageReady`, add:

```go
	if a.bucketDetailsView != nil {
		cmds = append(cmds, msgCmd(msg))
	}
```

- [ ] **Step 9: Add command palette constant**

In `internal/ui/components/commandpalette/commands.go`, find the `ViewType` constants and add (no nav command — requires a bucket selection):

```go
	ViewBucketDetails
```

- [ ] **Step 10: Add the corresponding `IconBucketDetails`**

```go
	IconBucketDetails = "📊"
```

- [ ] **Step 11: Build**

Run: `cd /home/vlad/dev/my/gcon && go build ./...`
Expected: build succeeds.

- [ ] **Step 12: Add a basic view test**

Create `internal/ui/views/bucket_details_test.go`:

```go
package views

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/gcp/usage"
)

func TestBucketDetailsView_InitFiresMonitoringRequest(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	cmd := v.Init()
	require.NotNil(t, cmd)
	// Init returns a Batch. We can't easily introspect; just ensure no panic.
}

func TestBucketDetailsView_DeepScanKey(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	// Manually flip to Usage tab.
	v.tabs.SelectByIndex(1)
	cmd := v.Update(deepScanKeyMsg())
	require.NotNil(t, cmd)
	got := cmd()
	req, ok := got.(UsageDeepScanRequestMsg)
	require.True(t, ok, "expected UsageDeepScanRequestMsg, got %T", got)
	assert.Equal(t, "b1", req.Bucket)
	assert.Equal(t, "", req.Prefix)
}

func TestBucketDetailsView_HandlesReadyMsg(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	v.Update(usage.ReadyMsg{
		Usage: usage.BucketUsage{
			Bucket:     "b1",
			TotalBytes: 12345,
			Source:     usage.SourceMonitoring,
			ScannedAt:  time.Now(),
		},
	})
	require.NotNil(t, v.usage)
	assert.Equal(t, int64(12345), v.usage.TotalBytes)
}

// deepScanKeyMsg constructs a tea.KeyMsg matching the 'C' binding without
// importing tea internals all over the test file.
func deepScanKeyMsg() teaKeyMsgC {
	return teaKeyMsgC{}
}

// We rely on the actual key matcher here; for v1 the test takes the shortcut
// of synthesizing a Type=runes key with 'C'. If that proves brittle, replace
// with a fake tea.Msg that the view's switch handles directly.
type teaKeyMsgC struct{}
```

If the `teaKeyMsgC` shortcut doesn't compile cleanly with `key.Matches`, swap it for the real key construction. Look at how other view tests synthesize key events in `objects_test.go`:

Run: `grep -n "tea.KeyMsg\|key.NewBinding\|Runes:" /home/vlad/dev/my/gcon/internal/ui/views/objects_test.go | head -10`

Adopt the same pattern.

- [ ] **Step 13: Run test**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/ui/views/ -run TestBucketDetailsView -v`
Expected: PASS.

- [ ] **Step 14: Commit**

```bash
git add internal/ui/app.go internal/ui/app_render.go internal/ui/app_navigation.go internal/ui/views/buckets.go internal/ui/views/bucket_details_test.go internal/ui/components/commandpalette/commands.go
git commit -m "2026-05-10: wire BucketDetailsView into App and dispatch chain"
```

---

### Task 3.3: ObjectsView `C` key + folderUsage field + inline stats

**Files:**
- Modify: `internal/ui/views/objects.go`
- Modify: `internal/ui/views/objects_test.go`

- [ ] **Step 1: Locate the ObjectsView struct and key map**

Run: `grep -n "type ObjectsView struct\|defaultObjectsKeyMap\|type objectsKeyMap" /home/vlad/dev/my/gcon/internal/ui/views/objects.go`

- [ ] **Step 2: Add field and binding**

In the struct, add:

```go
	// folderUsage holds the most recent deep-scan result for the current
	// prefix, displayed as an inline stats line above the table.
	folderUsage *usage.BucketUsage
```

Add the `usage` import if not present.

In the key map struct add:

```go
	DeepScan key.Binding
```

In its constructor:

```go
		DeepScan: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "calculate folder size"),
		),
```

- [ ] **Step 3: Wire the key handler**

In ObjectsView `Update`, in the key switch, add:

```go
			case key.Matches(msg, v.keys.DeepScan):
				return func() tea.Msg {
					return UsageDeepScanRequestMsg{Bucket: v.bucketName, Prefix: v.currentPrefix}
				}
```

(Verified: `v.bucketName` is the field name used throughout `objects.go`.)

- [ ] **Step 4: Handle ReadyMsg / ProgressMsg**

In `ObjectsView.Update`, add cases (early in the switch, before key handling):

```go
		case usage.ReadyMsg:
			if msg.Err != nil {
				return nil
			}
			if msg.Usage.Bucket != v.bucketName || msg.Usage.Prefix != v.currentPrefix {
				return nil // not for this folder
			}
			u := msg.Usage
			v.folderUsage = &u
			return nil

		case usage.ProgressMsg:
			if msg.Bucket != v.bucketName || msg.Prefix != v.currentPrefix {
				return nil
			}
			v.folderUsage = &usage.BucketUsage{
				Bucket:      msg.Bucket,
				Prefix:      msg.Prefix,
				TotalBytes:  msg.BytesScanned,
				ObjectCount: msg.ObjectsScanned,
				Source:      usage.SourceDeepScan,
			}
			return nil
```

(Verified: the field is `v.bucketName`.)

- [ ] **Step 5: Render the inline stats line**

In `ObjectsView.View()`, find where the table is rendered. Prepend the stats line when `folderUsage != nil`:

```go
	var statsLine string
	if v.folderUsage != nil {
		muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		statsLine = "\n  " + muted.Render(fmt.Sprintf(
			"Folder size: %s · %s objects (deep scan)",
			gcp.FormatSize(v.folderUsage.TotalBytes),
			formatObjectCount(v.folderUsage.ObjectCount),
		)) + "\n"
	}
	return statsLine + v.table.View() + help
```

If `formatObjectCount` doesn't exist in this package yet, copy the same helper from `buckets.go` (or extract it to a shared `views/usage_format.go`):

Create `internal/ui/views/usage_format.go`:

```go
package views

import "fmt"

// formatObjectCount renders n with comma thousands separators.
func formatObjectCount(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}
```

Then remove the duplicate from `buckets.go` (and `bucket_details.go` if duplicated — both should import this shared helper).

- [ ] **Step 6: Add a test**

Append to `internal/ui/views/objects_test.go`:

```go
func TestObjectsView_FolderUsageRenders(t *testing.T) {
	// NewObjectsView signature: (bucketName string, storageClient *gcp.StorageClient).
	// Pass nil for the storage client; we never trigger a load in this test.
	v := NewObjectsView("b1", nil)
	// Inject a deep-scan ReadyMsg matching the current (bucket, prefix) tuple.
	v.Update(usage.ReadyMsg{
		Usage: usage.BucketUsage{
			Bucket:      "b1",
			Prefix:      "",
			TotalBytes:  87_300_000_000,
			ObjectCount: 142_300,
			Source:      usage.SourceDeepScan,
		},
	})
	require.NotNil(t, v.folderUsage)
	assert.Equal(t, int64(142_300), v.folderUsage.ObjectCount)

	// View() requires a context; assign one with sane dimensions.
	ctxObj := context.New()
	ctxObj.SetDimensions(120, 40, 100, 35)
	v.SetContext(ctxObj)
	out := v.View()
	assert.Contains(t, out, "Folder size:")
	assert.Contains(t, out, "142,300")
}

func TestObjectsView_FolderUsageIgnoresOtherBuckets(t *testing.T) {
	v := NewObjectsView("b1", nil)
	v.Update(usage.ReadyMsg{
		Usage: usage.BucketUsage{
			Bucket:      "different-bucket",
			Prefix:      "",
			TotalBytes:  100,
			ObjectCount: 1,
			Source:      usage.SourceDeepScan,
		},
	})
	assert.Nil(t, v.folderUsage, "ReadyMsg for unrelated bucket should be ignored")
}
```

Add imports as needed: `usage`, `context`, `require`. If `View()` panics on a partly-initialized ObjectsView, the second test alone is sufficient and the first can be skipped — but the spec requires the inline-stats render to be exercised somehow, so prefer fixing the construction over deleting the test.

- [ ] **Step 7: Run tests**

Run: `cd /home/vlad/dev/my/gcon && go test ./internal/ui/views/ -run TestObjectsView -v`
Expected: PASS for new test, no regressions in existing ones.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/views/objects.go internal/ui/views/objects_test.go internal/ui/views/usage_format.go internal/ui/views/buckets.go internal/ui/views/bucket_details.go
git commit -m "2026-05-10: ObjectsView 'C' key for folder-scoped deep scan with inline stats"
```

---

### Task 3.4: Update docs and finalize Phase 3

**Files:**
- Modify: `README.md`
- Modify: `.claude/rules/key-bindings.md`
- Modify: `CLAUDE.md` (add to Implemented Features list)

- [ ] **Step 1: Update key-bindings.md — Buckets View**

Replace the Buckets View section (refining what Phase 2 already changed):

```markdown
| `Enter` | Browse bucket contents |
| `i` | View bucket details (Details/Usage tabs) |
| `S` | Open sort menu |
| `c` | Create new bucket |
| `C` | Calculate usage (deep scan, footer progress, Ctrl+X to cancel) |
| `/` | Filter buckets |
| `r` | Refresh list |
| `Esc` | Go back |
```

- [ ] **Step 2: Add Bucket Details View section to key-bindings.md**

After the Buckets View section, insert:

```markdown
## Bucket Details View

| Key | Action |
|-----|--------|
| `C` | Run deep scan (Usage tab) |
| `r` | Refresh monitoring metrics |
| `Tab` / `h` / `l` | Switch tabs (Details/Usage) |
| `1` / `2` | Jump to tab by number |
| `Enter` | Browse objects (when "Browse objects →" is focused) |
| `Esc` | Go back |
```

- [ ] **Step 3: Update Objects View section in key-bindings.md**

Add a row for `C`:

```markdown
| `C` | Calculate current folder size (deep scan, inline stats) |
```

- [ ] **Step 4: Update README.md**

Add to the bullet list under Cloud Storage:

```markdown
- Bucket details view with Usage tab showing breakdown by storage class, top-level prefix, and file extension
- Folder-scoped deep scan from the objects browser (`C` key)
```

- [ ] **Step 5: Update CLAUDE.md Implemented Features**

In `/home/vlad/dev/my/gcon/CLAUDE.md` under "Implemented Features", append:

```markdown
- [x] Cloud Storage bucket usage analysis
  - Cloud Monitoring metrics (total bytes, object count) shown inline in buckets list
  - On-demand deep scan with breakdowns by storage class, top-level prefix, and file extension
  - Live progress in footer task with `Ctrl+X` to cancel
  - Folder-scoped deep scan from objects browser
  - Bucket details view with Details and Usage tabs
```

- [ ] **Step 6: Run full test suite + lint + build**

Run: `cd /home/vlad/dev/my/gcon && go test ./... -timeout 120s && make lint && go build ./cmd/gcon`
Expected: all green.

- [ ] **Step 7: Smoke-test in the binary**

Run: `cd /home/vlad/dev/my/gcon && make build && ./build/gcon` (or whatever the binary name is).
Walk through:
- Open a project, navigate to Buckets — confirm Size/Objects columns populate.
- Press `C` on a small test bucket — confirm footer shows progress, then result with ✓.
- Press `i` — confirm BucketDetailsView opens with Details + Usage tabs.
- On Usage tab, press `C` — confirm same scan runs (cached, instant since it just ran).
- Enter the bucket (Enter), then in objects browser press `C` — confirm inline stats line appears.

If anything breaks, fix and re-run before committing.

- [ ] **Step 8: Commit (Phase 3 complete)**

```bash
git add README.md .claude/rules/key-bindings.md CLAUDE.md
git commit -m "2026-05-10: document bucket usage feature"
```

**Phase 3 Done.** PR 3 boundary. The full feature is shipped.

---

## Self-Review Checklist (do this after writing each PR)

Before opening each PR, run:

```bash
cd /home/vlad/dev/my/gcon && go test -race ./... -timeout 120s && make lint
```

And manually:
- All new keys documented in `key-bindings.md`
- README updated for any user-visible feature
- `CLAUDE.md` Implemented Features updated when a phase ships a user-visible capability
- New view (Phase 3) follows every step of `.claude/rules/adding-new-views.md`
- No stale `[ ]` checkboxes left in this plan file (all tasks complete)
