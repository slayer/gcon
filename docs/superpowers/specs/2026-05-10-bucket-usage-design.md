# Bucket Usage and Object Statistics — Design

**Date:** 2026-05-10
**Status:** Draft (approved for implementation planning)

## Goal

Surface storage usage and object counts for Cloud Storage buckets in `gcon`, with optional deep-scan breakdowns by storage class, top-level prefix, and file extension. Users should get fast at-a-glance numbers from Cloud Monitoring and accurate, real-time numbers (with breakdowns) on demand via a deep scan.

## Non-Goals

- No persistence of scan results to disk (session-only cache).
- No automatic deep scans — every deep scan is user-initiated via a key press.
- No project-wide rollup view (per-bucket and per-folder only).
- No semantic grouping of extensions (e.g. "media", "code"). Raw extensions only.
- No replacement of the existing object-browser flow. Deep scan is an addition.

## Background

GCS exposes no direct "bucket size" endpoint. Two viable sources:

| Source | Latency | Cost | Granularity | Freshness |
|--------|---------|------|-------------|-----------|
| Cloud Monitoring (`storage.googleapis.com/storage/total_bytes`, `.../object_count`) | Single API call per bucket (~ms) | Effectively free | Bucket-level only | Updated daily (~24h stale) |
| List + sum objects | Minutes for large buckets | Class A operations per 1k objects | Any prefix; can break down by storage class, prefix, extension | Real-time |

The design uses **both**: monitoring as the always-on default, deep scan as an opt-in "give me the truth" action.

## User-Facing Behavior

### BucketsView (buckets list)

Two new columns: `Size` and `Objects`. On view load, monitoring metrics are fetched in the background; cells render `…` until each bucket's data arrives. After a deep scan completes, those cells show the real-time numbers and gain a `✓` marker.

New key: **`C`** (capital) — start a deep scan of the selected bucket. Progress streams to the footer task. (Capital `C` is used everywhere for "Calculate usage" to avoid the `u`/upload conflict in ObjectsView and the `c`/create conflict in BucketsView.)

### BucketDetailsView (new)

Reached from BucketsView via:
- New key **`i`** (info), or
- Action menu entry "View bucket details" (via existing `.` key).

`Enter` on a bucket continues to mean **browse objects** (folder-like behavior, matching user expectation for a file/object browser). This deviates from the rest of gcon's "Enter → details" convention but is the right call for a bucket-as-folder mental model.

Two tabs:
- **Details** — name, location, storage class, creation date, versioning, lifecycle summary. A focusable "Browse objects →" link pushes ObjectsView.
- **Usage** — totals, breakdowns, scan controls.

Usage tab layout (single scrollable panel, not nested tabs):

```
Bucket: my-bucket                          (project: my-project)

Total size:    1.2 TB                      Source: Monitoring · 12h ago
Object count:  45,231                      Press 'C' to run a deep scan

— By Storage Class ————————————————————————
  STANDARD          980 GB        35,120
  NEARLINE          220 GB        10,111

— By Top-Level Prefix —————————————————————
  logs/             800 GB        30,000
  exports/          250 GB         8,500
  backups/           50 GB         1,200
  (root)            150 GB         5,531

— By Extension (top 20 by size) ——————————
  .parquet          600 GB        12,000
  .json.gz          250 GB        18,000
  .png               40 GB         8,500
  ...
```

Breakdown sections appear only after a deep scan has completed; before that, only the Monitoring totals are shown with a "Press C to scan" hint.

Keys on Usage tab:
- **`C`** — start (or restart) deep scan.
- **`r`** — refresh monitoring metrics (does not re-run deep scan).

### ObjectsView (existing — folder-scoped scan)

New key: **`C`** (capital) — start a deep scan of the **current prefix** (the folder the user is browsing). Lowercase `u` remains "upload"; `C` was free.

Result renders as an inline stats line at the top of the table:

```
Folder: logs/2026/                              ⏎ open · u rescan
─────────────────────────────────────────────────────────────────
  Folder size: 87.3 GB · 142,300 objects (deep scan, 2m ago)
─────────────────────────────────────────────────────────────────
[existing object table follows]
```

If no scan has been run for this prefix, the inline line is omitted entirely.

### Footer task progress

Active scans appear in the footer using the existing task system. Description updates live, debounced to ~500 ms:

```
⠋ Scanning my-bucket/logs/ — 142,300 objects · 87.3 GB
```

New global key **`Ctrl+X`**: when there is an active usage-scan task, cancel it. No-op otherwise. Scoped to scan tasks only — not a general "cancel any task" key.

Navigating away from the originating view does **not** cancel the scan. Results accumulate into the registry's cache and become visible to whichever view requests them next.

## Architecture

### Layer overview

```
┌────────────────────────────────────────────────────────────────┐
│ Views (BucketsView, BucketDetailsView, ObjectsView)            │
│   - Send: usage.RequestMsg{Bucket, Prefix, Mode}               │
│   - Receive: usage.ProgressMsg, usage.ReadyMsg                 │
└──────────────────────────┬─────────────────────────────────────┘
                           │  tea.Msg
                           ▼
┌────────────────────────────────────────────────────────────────┐
│ App.handleUsageRequest (in app_navigation.go)                  │
│   - Calls Scanner.FetchMonitoring or Scanner.StartDeepScan     │
│   - Wires footer task lifecycle (register / update / finish)   │
└──────────────────────────┬─────────────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────────────┐
│ internal/gcp/usage.Scanner                                     │
│   - cache:     map[key]BucketUsage                             │
│   - inflight:  map[key]*scanJob                                │
│   - Uses *gcp.StorageClient and *gcp.MonitoringClient          │
└──────────────────────────┬─────────────────────────────────────┘
                           │
            ┌──────────────┴──────────────┐
            ▼                             ▼
   StorageClient.ListAllObjects   MonitoringClient.FetchBucketUsage
   (existing, used as-is)         (NEW — joins total_bytes + object_count)
```

### `internal/gcp/usage` package

```go
package usage

type Source int
const (
    SourceMonitoring Source = iota
    SourceDeepScan
)

type Stat struct {
    Bytes int64
    Count int64
}

type BucketUsage struct {
    Bucket         string
    Prefix         string             // "" = whole bucket
    TotalBytes     int64
    ObjectCount    int64
    ByStorageClass map[string]Stat    // populated only when Source == SourceDeepScan
    ByTopPrefix    map[string]Stat    // populated only when Source == SourceDeepScan
    ByExtension    map[string]Stat    // populated only when Source == SourceDeepScan
    Source         Source
    ScannedAt      time.Time
}

type Scanner struct {
    storage    StorageLister              // interface (see below) — testability
    monitoring MonitoringFetcher          // interface — testability
    cache      map[string]BucketUsage
    inflight   map[string]*scanJob
    mu         sync.RWMutex
}

// Interfaces (defined in this package, satisfied by gcp.StorageClient / gcp.MonitoringClient)
type StorageLister interface {
    ListAllObjects(ctx context.Context, bucket, prefix string) ([]gcp.StorageObject, error)
    // Future: stream API to avoid loading all objects in memory at once.
    // For v1, ListAllObjects is acceptable — gcon already uses it elsewhere.
}
type MonitoringFetcher interface {
    FetchBucketUsage(ctx context.Context, project, bucket string) (bytes, count int64, asOf time.Time, err error)
}

func New(storage StorageLister, monitoring MonitoringFetcher) *Scanner

// Cache lookup. Returns the cached entry and true if present.
func (s *Scanner) Get(bucket, prefix string) (BucketUsage, bool)

// Returns a tea.Cmd that fetches monitoring metrics (one call) and emits ReadyMsg.
// No-op if a fresh enough monitoring entry is already cached.
func (s *Scanner) FetchMonitoring(ctx context.Context, project, bucket string) tea.Cmd

// Begins a deep scan and returns a jobID plus a "pump" tea.Cmd that reads exactly
// one message (Progress or Ready) from the job's internal channel.
//
// Streaming pattern: the scanner spawns a goroutine that performs the scan and
// pushes ProgressMsg values (debounced ~500ms) followed by a final ReadyMsg into
// a buffered channel. The returned tea.Cmd reads ONE message from that channel.
// The App.Update handler, on receiving a ProgressMsg for jobID, calls
// Scanner.NextMessage(jobID) and returns the resulting tea.Cmd to keep the pump
// alive. On ReadyMsg, the pump terminates naturally (channel closed).
//
// If a scan for the same (bucket, prefix) is already in flight, returns the existing
// jobID. Each call gets its own pump cmd reading from the same broadcast channel
// (the scanner fans out by spawning a new buffered subscriber channel per caller).
func (s *Scanner) StartDeepScan(ctx context.Context, bucket, prefix string) (jobID string, cmd tea.Cmd)

// NextMessage returns a tea.Cmd that reads the next message from the named job's
// channel. Used by the App handler to keep the pump running across messages.
// Returns a no-op cmd if jobID is unknown or completed.
func (s *Scanner) NextMessage(jobID string) tea.Cmd

func (s *Scanner) Cancel(jobID string)
func (s *Scanner) Invalidate(bucket, prefix string)

// Messages
type ProgressMsg struct {
    JobID          string
    Bucket, Prefix string
    ObjectsScanned int64
    BytesScanned   int64
}

type ReadyMsg struct {
    JobID string
    Usage BucketUsage   // valid only when Err == nil
    Err   error
}
```

**Cache key:** `bucket + "|" + prefix`. Monitoring entries use `prefix = ""`.

**Monitoring TTL:** 10 minutes in-memory. After that, `FetchMonitoring` triggers a re-fetch.

**Deep-scan TTL:** none; scan results live for the session and only get replaced when the user explicitly re-runs `C` (or `u`).

**In-flight dedup:** if `StartDeepScan` is called for a key already in `inflight`, the existing job's progress channel adds a new subscriber. This prevents two views (e.g. BucketsView + BucketDetailsView) from each spawning a scanner for the same bucket.

**Goroutine lifetime:** each scanJob owns one goroutine that reads from `ListAllObjects`. The goroutine sends progress through a buffered channel; a `tea.Cmd` polls that channel and converts to `tea.Msg`. On `Cancel`, the context is cancelled and the goroutine returns. On completion, the channel is closed.

**Memory footprint:** `ListAllObjects` returns the full slice. For a bucket with 10M objects, that's ~10GB of `StorageObject` structs. This is a known v1 limitation. The interface is designed so a future streaming API can replace `ListAllObjects` without changing callers. We will document the limit in the README and suggest scanning a prefix instead for very large buckets.

### MonitoringClient extension

Add to `internal/gcp/monitoring.go`:

```go
// FetchBucketUsage returns the most recent (within ~24h) total bytes and object
// count for a bucket. Both metrics are bucket-level; this method joins them
// via two ListTimeSeries calls and returns the matched values.
func (c *MonitoringClient) FetchBucketUsage(ctx context.Context, project, bucket string) (
    bytes int64, count int64, asOf time.Time, err error,
)
```

Filter for bytes:
```
metric.type="storage.googleapis.com/storage/total_bytes"
resource.type="gcs_bucket"
resource.labels.bucket_name="<bucket>"
```

Filter for objects: same with `metric.type="storage.googleapis.com/storage/object_count"`.

Aligner: `ALIGN_MEAN`. Time range: last 36 hours (covers the daily metric publication window even on lazy buckets).

Returns the most recent point's value and timestamp. If no points exist (e.g. a brand-new bucket), returns `(0, 0, zeroTime, nil)` — callers should treat zero `asOf` as "no data yet".

### Footer task helper

Add to `internal/ui/app.go`:

```go
// updateRunningTask updates the description of an already-running task.
// Used to surface live progress (e.g. scan counters) without re-registering the task.
// No-op if the task is not in TaskRunning state.
func (a *App) updateRunningTask(id, description string) {
    if t, ok := a.ctx.Tasks[id]; ok && t.State == context.TaskRunning {
        t.Description = description
        a.ctx.Tasks[id] = t
    }
}
```

This is the only change to the task system. `registerRunningTask` and `finishTask` work unchanged.

### App-level wiring

The Scanner is constructed once in `App.Init()` after the Storage and Monitoring clients are ready. It lives on the `App` struct:

```go
type App struct {
    // ... existing fields
    usageScanner *usage.Scanner
}
```

New message handlers in `app_navigation.go`:

```go
// View → "I want monitoring data for this bucket"
case views.UsageMonitoringRequestMsg:
    return a.handleUsageMonitoringRequest(msg)

// View → "Start a deep scan"
case views.UsageDeepScanRequestMsg:
    return a.handleUsageDeepScanRequest(msg)

// Scanner → progress (also re-arms the pump for the next message)
case usage.ProgressMsg:
    a.updateRunningTask(scanTaskID(msg.JobID), formatProgress(msg))
    return tea.Batch(
        a.usageScanner.NextMessage(msg.JobID),  // keep the pump alive
        a.dispatchUsageProgress(msg),            // forward to interested view(s)
    )

// Scanner → done (pump terminates naturally; no NextMessage needed)
case usage.ReadyMsg:
    finishCmd := a.finishTask(scanTaskID(msg.JobID), msg.Err)
    return tea.Batch(finishCmd, a.dispatchUsageReady(msg))
```

`dispatchUsageProgress` / `dispatchUsageReady` route the message to `bucketsView`, `bucketDetailsView`, and `objectsView` if they exist and care about that bucket/prefix.

New global key in `keys.go`:

```go
CancelUsageScan: key.NewBinding(
    key.WithKeys("ctrl+x"),
    key.WithHelp("ctrl+x", "cancel scan"),
),
```

Handler in `app.go`:
```go
case key.Matches(msg, a.keys.CancelUsageScan):
    if jobID, ok := a.activeUsageScanJobID(); ok {
        a.usageScanner.Cancel(jobID)
        return a, nil
    }
```

`activeUsageScanJobID()` walks `ctx.Tasks` looking for a task ID matching the `usage-scan-*` prefix.

### View changes

**BucketsView** (`internal/ui/views/buckets.go`):
- Add `Size` and `Objects` columns (~10 chars each), update `bucketColumns()` and `bucketToRow()`.
- After `bucketsLoadedMsg`, emit one `UsageMonitoringRequestMsg` per bucket via `tea.Batch`.
- Handle `usage.ReadyMsg` and `usage.ProgressMsg` to update the corresponding row's cells in place. Cells default to `…` and a faint placeholder until first message.
- Add `C` key → `UsageDeepScanRequestMsg{Bucket, Prefix: ""}`.
- Add `i` key → `BucketDetailsRequestMsg{Bucket}` (new message).
- Update help text.

**BucketDetailsView** (NEW — `internal/ui/views/bucket_details.go`):
- Embeds `tabs.Model` with two tabs: Details, Usage.
- Implements `View`, `Update`, `Init`, `SetSize`, `SetContext`, `HasTextInputFocused` (returns false — no text input).
- Implements `IsMenuOpen` for the action menu.
- On `Init`, fires `UsageMonitoringRequestMsg` if cache is empty for this bucket.
- Keys: `i` no-op (already here), `C` deep scan, `Tab` switch tab focus, `1`/`2`/`h`/`l` switch tabs, `r` refresh monitoring.
- Action menu entries: "Run deep scan", "Refresh monitoring", "Browse objects".
- Implements all 16 steps from `adding-new-views.md` (constant, app field, render switch, update handlers, navigation, clearAllViews, sidebar update, command palette entry).

**ObjectsView** (existing — `internal/ui/views/objects.go`):
- Add `C` key → `UsageDeepScanRequestMsg{Bucket, Prefix: v.currentPrefix}`.
- Add internal field `folderUsage *usage.BucketUsage`.
- Handle `usage.ReadyMsg` filtered by matching bucket+prefix; store and re-render.
- View() prepends the inline stats line above the existing table when `folderUsage != nil`.

**Action menu** entries on BucketsView: add "Run deep scan (C)", "View bucket details (i)".

### Sidebar / command palette

Per `adding-new-views.md` step 10, add to `commandpalette/commands.go`:
- New `ViewBucketDetails` constant (no sidebar entry — accessed only via parent).
- Command `nav:bucket-details` is **not** added to NavigationCommands (it requires a bucket selection — not a top-level navigation).

The Buckets sidebar entry remains as-is.

## Testing Strategy

### Unit tests (`internal/gcp/usage/`)

- `scanner_test.go`:
  - Cache hit returns immediately, no client calls.
  - Cache miss → fetch → cache populated.
  - Two concurrent `StartDeepScan` for same key → one job, both subscribers receive messages.
  - `Cancel` → goroutine exits, `ReadyMsg{Err: context.Canceled}` emitted.
  - Tally correctness: feed canned `[]StorageObject` slices, assert `BucketUsage` totals + breakdowns.
  - Empty bucket → zero values, no error.
  - Top-prefix grouping: objects with no `/` go under `(root)`; objects with one `/` group by prefix; deeper paths roll up to first segment.
  - Extension grouping: handles dotless filenames, double extensions (`.tar.gz`), case-insensitive (`.JPG` == `.jpg`).
- `monitoring_bucket_usage_test.go`: table-driven against a fake `MetricClient` (existing pattern from `monitoring_cloudrun_test.go`). Cases: both metrics present, only bytes present (object_count missing → return 0 count), no points at all (return zero asOf).

### View tests

- `BucketsView` (`buckets_test.go` extension):
  - Simulate `usage.ReadyMsg` for one bucket → assert that bucket's row updates.
  - `u` key emits `UsageDeepScanRequestMsg{Bucket, ""}`.
  - `i` key emits `BucketDetailsRequestMsg{Bucket}`.
- `BucketDetailsView` (`bucket_details_test.go` new):
  - Init triggers monitoring fetch.
  - `C` triggers deep scan request.
  - After `usage.ReadyMsg` with deep scan, breakdown sections render with correct content.
  - Tab switching works.
- `ObjectsView` (`objects_test.go` extension):
  - `u` emits scan request scoped to current prefix.
  - After ready msg matching current prefix, inline stats line renders.
  - Ready msg for a different prefix is ignored.

### App-level test

- `app_usage_test.go` (new): wire a fake Scanner (interface-extracted) into App, simulate views requesting monitoring/deep scan, assert footer task lifecycle (register → update → finish), assert `Ctrl+X` triggers Cancel on active scan.

### Out of scope for v1 tests

- No integration test against a real GCS or fake server. The interfaces decouple from the SDK well enough that unit + view tests cover behavior.

## Phasing

### PR 1 — Data layer (no UI changes)
- `internal/gcp/usage` package with `Scanner`, `BucketUsage`, message types.
- `MonitoringClient.FetchBucketUsage`.
- `App.updateRunningTask` helper.
- Full test coverage for the package.
- **Reviewable in isolation.** No user-visible changes yet.

### PR 2 — Buckets list integration
- New `Size` / `Objects` columns in BucketsView.
- Scanner initialized in App, monitoring fetch on bucket-list load.
- `u` key for whole-bucket deep scan with footer progress.
- `Ctrl+X` cancel.
- Action menu entry "Run deep scan".
- Updated `key-bindings.md` and `README.md`.
- **First user-visible value.**

### PR 3 — BucketDetailsView + ObjectsView folder scan
- New BucketDetailsView with Details/Usage tabs.
- `i` key on BucketsView, action menu entry, command palette entry.
- All 16 steps from `adding-new-views.md`.
- ObjectsView `u` key + inline stats line.
- "Browse objects →" link in Details tab.
- Updated docs.

## Open Risks

1. **Memory pressure on huge bucket scans.** `ListAllObjects` returns the full slice. Mitigation: documented limit, `Cancel` always available, recommend prefix-scoped scans. Future: streaming `ListAllObjectsStream(ctx) <-chan StorageObject` API.
2. **Monitoring metric staleness.** ~24h. Surfaced in UI with "12h ago" hint so users aren't surprised. Refresh button available.
3. **Deep-scan cost.** Class A operations are billed per 1000 objects. We never run automatically; every scan is opt-in. Should still call it out in README.
4. **Footer task description length.** Long descriptions could overflow the footer. Mitigation: scanner-side formatter caps the description at ~60 chars and truncates the bucket/prefix if needed.
5. **Scanner outliving views.** When the user switches projects, `clearAllViews()` runs but the Scanner cache and any in-flight scans persist. We should also clear/cancel them in `clearAllViews()`. (Added to PR 2 work.)
