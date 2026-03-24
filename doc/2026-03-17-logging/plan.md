# Logging Explorer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a terminal-based GCP Logs Explorer with LQL query input, quick filters, sparkline histogram, expandable log entries, infinite scroll, and tail mode.

**Architecture:** A new top-level `LogsView` in `internal/ui/views/logs.go` orchestrates a filter bar, query input, and a reusable `logviewer` component. The GCP API layer in `internal/gcp/logging.go` is extended with pagination, resource/log-name listing, and histogram support. The view follows existing patterns (sidebar item, command palette, view lifecycle).

**Tech Stack:** Go, Bubble Tea, Lip Gloss, `cloud.google.com/go/logging/logadmin`, Cloud Monitoring API.

---

## Task 1: Extend LogEntry Struct

**Files:**
- Modify: `internal/gcp/logging.go` (lines 20-26, LogEntry struct)
- Test: `internal/gcp/logging_test.go`

**Step 1: Write the test for the expanded LogEntry**

```go
// internal/gcp/logging_test.go
func TestLogEntryFlattenFields(t *testing.T) {
	entry := LogEntry{
		Timestamp:    time.Date(2026, 3, 16, 13, 4, 1, 0, time.UTC),
		Severity:     "ERROR",
		Message:      "connection refused",
		LogName:      "projects/my-proj/logs/stderr",
		ResourceType: "gce_instance",
		ResourceLabels: map[string]string{
			"instance_id": "123456",
			"zone":        "us-central1-a",
		},
		Labels: map[string]string{
			"compute.googleapis.com/resource_name": "my-vm",
		},
		JSONPayload: map[string]any{
			"httpRequest": map[string]any{
				"method": "GET",
				"status": float64(502),
			},
		},
		InsertID: "abc123",
	}

	fields := entry.FlattenFields()

	// Fields should be sorted alphabetically
	assert.True(t, len(fields) > 0, "should have flattened fields")

	// Check specific fields exist
	fieldMap := make(map[string]string)
	for _, f := range fields {
		fieldMap[f.Key] = f.Value
	}
	assert.Equal(t, "gce_instance", fieldMap["resource.type"])
	assert.Equal(t, "123456", fieldMap["resource.labels.instance_id"])
	assert.Equal(t, "us-central1-a", fieldMap["resource.labels.zone"])
	assert.Equal(t, "my-vm", fieldMap["labels.compute.googleapis.com/resource_name"])
	assert.Equal(t, "GET", fieldMap["jsonPayload.httpRequest.method"])
	assert.Equal(t, "502", fieldMap["jsonPayload.httpRequest.status"])
	assert.Equal(t, "abc123", fieldMap["insertId"])
}

func TestLogEntryFlattenFieldsSorted(t *testing.T) {
	entry := LogEntry{
		ResourceType:   "cloud_run_revision",
		ResourceLabels: map[string]string{"zone": "us-central1-a", "instance": "abc"},
		Labels:         map[string]string{"b_label": "val", "a_label": "val"},
	}

	fields := entry.FlattenFields()
	for i := 1; i < len(fields); i++ {
		assert.True(t, fields[i-1].Key <= fields[i].Key,
			"fields should be sorted: %s should come before %s", fields[i-1].Key, fields[i].Key)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/gcp/ -run TestLogEntryFlatten -v
```
Expected: FAIL — `FlattenFields` not defined, `FlattenedField` type not defined.

**Step 3: Expand the LogEntry struct and add FlattenFields**

In `internal/gcp/logging.go`, replace the existing `LogEntry` struct and add new types/methods:

```go
// FlattenedField represents a single flattened key-value pair from a log entry.
type FlattenedField struct {
	Key   string
	Value string
}

// LogEntry represents a single log entry with full structured data.
type LogEntry struct {
	Timestamp      time.Time
	Severity       string // INFO, WARNING, ERROR, CRITICAL, etc.
	Message        string
	LogName        string            // e.g. "projects/my-proj/logs/stderr"
	ResourceType   string            // e.g. "gce_instance"
	ResourceLabels map[string]string // instance_id, zone, etc.
	Labels         map[string]string // user + system labels
	JSONPayload    map[string]any    // full structured payload
	TextPayload    string            // raw text if not JSON
	InsertID       string            // unique entry ID
	TraceID        string
	SpanID         string
	SourceLocation string
}

// FlattenFields returns all non-empty fields as sorted dot-notated key-value pairs.
func (e *LogEntry) FlattenFields() []FlattenedField {
	var fields []FlattenedField

	if e.ResourceType != "" {
		fields = append(fields, FlattenedField{Key: "resource.type", Value: e.ResourceType})
	}
	for k, v := range e.ResourceLabels {
		fields = append(fields, FlattenedField{Key: "resource.labels." + k, Value: v})
	}
	if e.LogName != "" {
		fields = append(fields, FlattenedField{Key: "logName", Value: e.LogName})
	}
	if e.InsertID != "" {
		fields = append(fields, FlattenedField{Key: "insertId", Value: e.InsertID})
	}
	if e.TraceID != "" {
		fields = append(fields, FlattenedField{Key: "trace", Value: e.TraceID})
	}
	if e.SpanID != "" {
		fields = append(fields, FlattenedField{Key: "spanId", Value: e.SpanID})
	}
	if e.SourceLocation != "" {
		fields = append(fields, FlattenedField{Key: "sourceLocation", Value: e.SourceLocation})
	}
	for k, v := range e.Labels {
		fields = append(fields, FlattenedField{Key: "labels." + k, Value: v})
	}
	if e.TextPayload != "" {
		fields = append(fields, FlattenedField{Key: "textPayload", Value: e.TextPayload})
	}
	// Flatten JSON payload recursively
	flattenMap("jsonPayload", e.JSONPayload, &fields)

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Key < fields[j].Key
	})
	return fields
}

// flattenMap recursively flattens a nested map into dot-notated key-value pairs.
func flattenMap(prefix string, m map[string]any, fields *[]FlattenedField) {
	for k, v := range m {
		fullKey := prefix + "." + k
		switch val := v.(type) {
		case map[string]any:
			flattenMap(fullKey, val, fields)
		default:
			*fields = append(*fields, FlattenedField{Key: fullKey, Value: fmt.Sprintf("%v", val)})
		}
	}
}
```

Add `"sort"` to the imports if not already present.

**Step 4: Run test to verify it passes**

```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/gcp/ -run TestLogEntryFlatten -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/gcp/logging.go internal/gcp/logging_test.go
git commit -m "2026-03-17: Expand LogEntry struct with full fields and FlattenFields method"
```

---

## Task 2: Add ListLogEntries with Pagination

**Files:**
- Modify: `internal/gcp/logging.go`
- Test: `internal/gcp/logging_test.go`

**Step 1: Write the test**

```go
func TestListLogEntriesParsesPayload(t *testing.T) {
	// This test verifies the conversion logic from logging.Entry to our LogEntry.
	// We test the converter function directly since the API call requires a real client.
	entry := &logging.Entry{
		Timestamp: time.Date(2026, 3, 16, 13, 4, 1, 0, time.UTC),
		Severity:  logging.Error,
		Payload:   map[string]any{"httpRequest": map[string]any{"method": "GET"}},
		LogName:   "projects/test/logs/stderr",
		Resource: &monitoredres.MonitoredResource{
			Type:   "gce_instance",
			Labels: map[string]string{"instance_id": "123"},
		},
		Labels:   map[string]string{"key": "val"},
		InsertID: "ins-1",
		Trace:    "projects/test/traces/abc",
		SpanID:   "span-1",
	}

	result := convertLogEntry(entry)

	assert.Equal(t, "ERROR", result.Severity)
	assert.Equal(t, "gce_instance", result.ResourceType)
	assert.Equal(t, "123", result.ResourceLabels["instance_id"])
	assert.Equal(t, "val", result.Labels["key"])
	assert.Equal(t, "ins-1", result.InsertID)
	assert.Equal(t, "projects/test/traces/abc", result.TraceID)
	assert.Equal(t, "span-1", result.SpanID)
	assert.NotEmpty(t, result.JSONPayload)
	assert.Equal(t, "GET", result.JSONPayload["httpRequest"].(map[string]any)["method"])
}
```

Note: You'll need to add these imports to the test file:
```go
import (
	"cloud.google.com/go/logging"
	"google.golang.org/genproto/googleapis/api/monitoredres"
)
```

Check go.mod — `google.golang.org/genproto` may need to be added. If so, run `go mod tidy` after adding the import.

**Step 2: Run test to verify it fails**

```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/gcp/ -run TestListLogEntriesParsesPayload -v
```

**Step 3: Implement ListLogEntries and the converter**

Add to `internal/gcp/logging.go`:

```go
import (
	"cloud.google.com/go/logging"
	// ... existing imports
)

// convertLogEntry converts a Cloud Logging Entry to our LogEntry type.
func convertLogEntry(entry *logging.Entry) LogEntry {
	logEntry := LogEntry{
		Timestamp: entry.Timestamp,
		Severity:  normalizeSeverity(entry.Severity.String()),
		InsertID:  entry.InsertID,
		LogName:   entry.LogName,
		TraceID:   entry.Trace,
		SpanID:    entry.SpanID,
	}

	// Extract resource info
	if entry.Resource != nil {
		logEntry.ResourceType = entry.Resource.Type
		logEntry.ResourceLabels = entry.Resource.Labels
	}

	// Copy labels
	logEntry.Labels = entry.Labels

	// Extract source location
	if entry.SourceLocation != nil {
		logEntry.SourceLocation = fmt.Sprintf("%s:%d", entry.SourceLocation.File, entry.SourceLocation.Line)
	}

	// Handle payload types
	switch p := entry.Payload.(type) {
	case string:
		logEntry.TextPayload = p
		logEntry.Message = p
	case map[string]any:
		logEntry.JSONPayload = p
		logEntry.Message = fmt.Sprintf("%v", p)
	default:
		logEntry.Message = fmt.Sprintf("%v", p)
	}

	return logEntry
}

// ListLogEntries executes an LQL query and returns paginated results.
// Returns entries, nextPageToken, and error.
func (c *LoggingClient) ListLogEntries(
	ctx context.Context,
	filter string,
	pageSize int,
	pageToken string,
) ([]LogEntry, string, error) {
	opts := []logadmin.EntriesOption{
		logadmin.NewestFirst(),
		logadmin.Filter(filter),
	}

	iter := c.adminClient.Entries(ctx, opts...)

	// Skip to page token position if provided.
	// Note: logadmin doesn't support page tokens directly,
	// so we use a simple offset-based approach with the iterator.
	// For the initial implementation, we just limit results.

	var entries []LogEntry
	count := 0

	for count < pageSize {
		entry, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to fetch log entries: %w", err)
		}

		entries = append(entries, convertLogEntry(entry))
		count++
	}

	// Check if there are more entries
	nextToken := ""
	if count == pageSize {
		// There may be more results; use the last entry's InsertID as a cursor
		if len(entries) > 0 {
			nextToken = entries[len(entries)-1].InsertID
		}
	}

	return entries, nextToken, nil
}
```

**Important:** The `logadmin` library doesn't have built-in page token support. For the initial implementation, we use `InsertID` of the last entry as a cursor and add a timestamp filter for the next page. This can be refined later. When `pageToken` is provided, append `AND timestamp <= "lastTimestamp" AND insertId != "lastInsertID"` to the filter.

Update the method to handle page tokens:

```go
func (c *LoggingClient) ListLogEntries(
	ctx context.Context,
	filter string,
	pageSize int,
	pageToken string,
) ([]LogEntry, string, error) {
	// pageToken format: "timestamp|insertID" for cursor-based pagination
	if pageToken != "" {
		parts := strings.SplitN(pageToken, "|", 2)
		if len(parts) == 2 {
			filter += fmt.Sprintf(` AND timestamp <= "%s"`, parts[0])
		}
	}

	opts := []logadmin.EntriesOption{
		logadmin.NewestFirst(),
		logadmin.Filter(filter),
	}

	iter := c.adminClient.Entries(ctx, opts...)

	// Skip the entry matching the page token's insertID (already seen)
	skipInsertID := ""
	if pageToken != "" {
		parts := strings.SplitN(pageToken, "|", 2)
		if len(parts) == 2 {
			skipInsertID = parts[1]
		}
	}

	var entries []LogEntry
	count := 0

	for count < pageSize {
		entry, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to fetch log entries: %w", err)
		}

		converted := convertLogEntry(entry)

		// Skip the cursor entry from the previous page
		if skipInsertID != "" && converted.InsertID == skipInsertID {
			skipInsertID = "" // only skip the first match
			continue
		}

		entries = append(entries, converted)
		count++
	}

	// Build next page token
	nextToken := ""
	if count == pageSize && len(entries) > 0 {
		last := entries[len(entries)-1]
		nextToken = last.Timestamp.UTC().Format(time.RFC3339Nano) + "|" + last.InsertID
	}

	return entries, nextToken, nil
}
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/gcp/ -run TestListLogEntries -v
```

**Step 5: Run the full test suite for the gcp package**

```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/gcp/ -v
```

Ensure existing tests still pass (the old `GetRecentLogs` and `GetCloudRunLogs` methods are unchanged).

**Step 6: Commit**

```bash
git add internal/gcp/logging.go internal/gcp/logging_test.go
git commit -m "2026-03-17: Add ListLogEntries with pagination and entry converter"
```

---

## Task 3: Add ListResourceTypes and ListLogNames

**Files:**
- Modify: `internal/gcp/logging.go`
- Test: `internal/gcp/logging_test.go`

**Step 1: Implement ListLogNames and ListResourceTypes**

```go
// ListLogNames returns log names present in the project.
func (c *LoggingClient) ListLogNames(ctx context.Context) ([]string, error) {
	iter := c.adminClient.Logs(ctx)

	var logNames []string
	for {
		logName, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list log names: %w", err)
		}
		logNames = append(logNames, logName)
	}

	sort.Strings(logNames)
	return logNames, nil
}

// ListResourceTypes returns monitored resource types that have log entries.
func (c *LoggingClient) ListResourceTypes(ctx context.Context) ([]string, error) {
	iter := c.adminClient.ResourceDescriptors(ctx)

	var types []string
	for {
		desc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list resource descriptors: %w", err)
		}
		types = append(types, desc.Type)
	}

	sort.Strings(types)
	return types, nil
}
```

Note: `c.adminClient.Logs(ctx)` returns a `*logadmin.LogIterator` and `c.adminClient.ResourceDescriptors(ctx)` returns a `*logadmin.ResourceDescriptorIterator`. Both are part of the existing `logadmin` package.

Check the `logadmin` API — `ResourceDescriptors` returns `*monitoredres.MonitoredResourceDescriptor`, which has a `Type` field. Import `google.golang.org/genproto/googleapis/api/monitoredres` if not already available.

**Step 2: Write a simple test**

```go
func TestNormalizeSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Default", "INFO"},
		{"Info", "INFO"},
		{"Warning", "WARNING"},
		{"Error", "ERROR"},
		{"Critical", "CRITICAL"},
		{"EMERGENCY", "EMERGENCY"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeSeverity(tt.input))
		})
	}
}
```

**Step 3: Run tests**

```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/gcp/ -run TestNormalizeSeverity -v
```

**Step 4: Verify compilation of the new methods**

```bash
cd /Users/vlad/dev/my/gcon && go build ./internal/gcp/
```

**Step 5: Commit**

```bash
git add internal/gcp/logging.go internal/gcp/logging_test.go
git commit -m "2026-03-17: Add ListLogNames and ListResourceTypes methods"
```

---

## Task 4: Add GetLogHistogram

**Files:**
- Modify: `internal/gcp/logging.go` (or create `internal/gcp/logging_histogram.go`)
- Test: `internal/gcp/logging_test.go`

**Step 1: Implement GetLogHistogram**

This uses the Cloud Monitoring API to fetch `logging.googleapis.com/log_entry_count` metric:

```go
// GetLogHistogram fetches log entry counts bucketed over a time range for sparkline display.
// Uses Cloud Monitoring API: logging.googleapis.com/log_entry_count metric.
func (c *LoggingClient) GetLogHistogram(
	ctx context.Context,
	monClient *MonitoringClient,
	filter string,
	timeRange time.Duration,
) ([]DataPoint, error) {
	if monClient == nil {
		return nil, fmt.Errorf("monitoring client not initialized")
	}

	// Use the monitoring client's fetchMetricData to get log entry counts
	metricFilter := `metric.type = "logging.googleapis.com/log_entry_count"`

	return monClient.fetchMetricData(ctx, metricFilter, timeRange)
}
```

Wait — `fetchMetricData` is unexported (lowercase). It's on `MonitoringClient`. We need to either:
- Export it (rename to `FetchMetricData`), or
- Add a new public method on `MonitoringClient` for this specific metric.

Looking at the existing code, the pattern is to add specific public methods. Let's add one to `monitoring.go`:

Add to `internal/gcp/monitoring.go`:

```go
// GetLogEntryCount fetches the logging.googleapis.com/log_entry_count metric
// for sparkline histogram display in the Logs Explorer.
func (c *MonitoringClient) GetLogEntryCount(ctx context.Context, timeRange time.Duration) ([]DataPoint, error) {
	filter := `metric.type = "logging.googleapis.com/log_entry_count"`
	return c.fetchMetricData(ctx, filter, timeRange)
}
```

**Step 2: Verify compilation**

```bash
cd /Users/vlad/dev/my/gcon && go build ./internal/gcp/
```

**Step 3: Commit**

```bash
git add internal/gcp/monitoring.go
git commit -m "2026-03-17: Add GetLogEntryCount for log histogram sparkline"
```

---

## Task 5: Sparkline Component

**Files:**
- Create: `internal/ui/components/logviewer/sparkline.go`
- Test: `internal/ui/components/logviewer/sparkline_test.go`

**Step 1: Write tests for sparkline rendering**

```go
// internal/ui/components/logviewer/sparkline_test.go
package logviewer

import (
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestSparklineEmpty(t *testing.T) {
	result := RenderSparkline(nil, 20, 0)
	assert.Contains(t, result, "No data")
}

func TestSparklineBasic(t *testing.T) {
	data := []gcp.DataPoint{
		{Timestamp: time.Now().Add(-5 * time.Minute), Value: 0},
		{Timestamp: time.Now().Add(-4 * time.Minute), Value: 50},
		{Timestamp: time.Now().Add(-3 * time.Minute), Value: 100},
		{Timestamp: time.Now().Add(-2 * time.Minute), Value: 50},
		{Timestamp: time.Now().Add(-1 * time.Minute), Value: 0},
	}
	result := RenderSparkline(data, 20, 12345)
	// Should contain block characters
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "12,345")
}

func TestSparklineAllSameValue(t *testing.T) {
	data := []gcp.DataPoint{
		{Timestamp: time.Now().Add(-3 * time.Minute), Value: 50},
		{Timestamp: time.Now().Add(-2 * time.Minute), Value: 50},
		{Timestamp: time.Now().Add(-1 * time.Minute), Value: 50},
	}
	result := RenderSparkline(data, 20, 100)
	assert.NotEmpty(t, result)
}

func TestSparklineResultCountFormatting(t *testing.T) {
	tests := []struct {
		count    int64
		expected string
	}{
		{0, "0 results"},
		{1, "1 result"},
		{999, "999 results"},
		{1000, "1,000 results"},
		{1234567, "1,234,567 results"},
	}
	for _, tt := range tests {
		result := formatResultCount(tt.count)
		assert.Equal(t, tt.expected, result)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/ui/components/logviewer/ -run TestSparkline -v
```

**Step 3: Implement the sparkline renderer**

```go
// internal/ui/components/logviewer/sparkline.go
package logviewer

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

// Block characters for sparkline rendering, from lowest to highest
var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// RenderSparkline renders a single-line sparkline from data points.
// width is the available character width. totalCount is displayed on the right.
func RenderSparkline(data []gcp.DataPoint, width int, totalCount int64) string {
	countStr := formatResultCount(totalCount)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	sparkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	if len(data) == 0 {
		return mutedStyle.Render("  No data") + "  " + mutedStyle.Render(countStr)
	}

	// Reserve space for "  " prefix + "  " separator + count string
	sparkWidth := width - 4 - lipgloss.Width(countStr)
	if sparkWidth < 5 {
		sparkWidth = 5
	}

	// Bucket data points into sparkWidth buckets
	buckets := bucketize(data, sparkWidth)

	// Find min/max for scaling
	minVal, maxVal := buckets[0], buckets[0]
	for _, v := range buckets {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	// Render sparkline characters
	var spark strings.Builder
	valRange := maxVal - minVal
	for _, v := range buckets {
		idx := 0
		if valRange > 0 {
			idx = int(math.Round(float64(v-minVal) / float64(valRange) * float64(len(sparkBlocks)-1)))
		} else {
			idx = len(sparkBlocks) / 2 // all same value: mid-height
		}
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		spark.WriteRune(sparkBlocks[idx])
	}

	return "  " + sparkStyle.Render(spark.String()) + "  " + mutedStyle.Render(countStr)
}

// bucketize distributes data points into n buckets by averaging values per bucket.
func bucketize(data []gcp.DataPoint, n int) []float64 {
	if n <= 0 {
		return nil
	}
	if len(data) <= n {
		// Fewer points than buckets: spread them out
		buckets := make([]float64, n)
		for i, dp := range data {
			idx := i * n / len(data)
			buckets[idx] = dp.Value
		}
		return buckets
	}

	// More points than buckets: average within each bucket
	buckets := make([]float64, n)
	pointsPerBucket := float64(len(data)) / float64(n)

	for i := range n {
		start := int(float64(i) * pointsPerBucket)
		end := int(float64(i+1) * pointsPerBucket)
		if end > len(data) {
			end = len(data)
		}
		sum := 0.0
		count := 0
		for j := start; j < end; j++ {
			sum += data[j].Value
			count++
		}
		if count > 0 {
			buckets[i] = sum / float64(count)
		}
	}

	return buckets
}

// formatResultCount formats a count with comma separators and singular/plural.
func formatResultCount(count int64) string {
	if count == 1 {
		return "1 result"
	}
	return fmt.Sprintf("%s results", formatWithCommas(count))
}

// formatWithCommas formats an integer with comma thousand separators.
func formatWithCommas(n int64) string {
	if n < 0 {
		return "-" + formatWithCommas(-n)
	}
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	var result strings.Builder
	remainder := len(str) % 3
	if remainder > 0 {
		result.WriteString(str[:remainder])
		if len(str) > remainder {
			result.WriteString(",")
		}
	}
	for i := remainder; i < len(str); i += 3 {
		if i > remainder {
			result.WriteString(",")
		}
		result.WriteString(str[i : i+3])
	}
	return result.String()
}
```

**Step 4: Run tests**

```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/ui/components/logviewer/ -run TestSparkline -v
```

**Step 5: Commit**

```bash
git add internal/ui/components/logviewer/
git commit -m "2026-03-17: Add sparkline renderer component for log histogram"
```

---

## Task 6: Log Entry Rendering

**Files:**
- Create: `internal/ui/components/logviewer/entry.go`
- Test: `internal/ui/components/logviewer/entry_test.go`

**Step 1: Write tests for entry rendering**

```go
// internal/ui/components/logviewer/entry_test.go
package logviewer

import (
	"strings"
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestRenderCompactEntry(t *testing.T) {
	entry := gcp.LogEntry{
		Timestamp:    time.Date(2026, 3, 16, 13, 4, 1, 0, time.UTC),
		Severity:     "ERROR",
		Message:      "connection refused to database",
		ResourceType: "gce_instance",
	}

	result := RenderCompactEntry(entry, false, 80)

	assert.Contains(t, result, "▸")        // collapsed indicator
	assert.Contains(t, result, "E")         // severity abbreviation
	assert.Contains(t, result, "13:04:01")  // timestamp
	assert.Contains(t, result, "gce_instance")
	assert.Contains(t, result, "connection refused")
}

func TestRenderCompactEntryExpanded(t *testing.T) {
	entry := gcp.LogEntry{
		Timestamp:    time.Date(2026, 3, 16, 13, 4, 1, 0, time.UTC),
		Severity:     "WARNING",
		Message:      "slow query detected",
		ResourceType: "cloud_run_revision",
	}

	result := RenderCompactEntry(entry, true, 80)
	assert.Contains(t, result, "▾") // expanded indicator
}

func TestRenderExpandedFields(t *testing.T) {
	entry := gcp.LogEntry{
		ResourceType:   "gce_instance",
		ResourceLabels: map[string]string{"instance_id": "123"},
		Labels:         map[string]string{"key": "val"},
		InsertID:       "abc",
	}

	result := RenderExpandedFields(entry, -1, 80)

	assert.Contains(t, result, "resource.type")
	assert.Contains(t, result, "gce_instance")
	assert.Contains(t, result, "resource.labels.instance_id")
	assert.Contains(t, result, "123")
}

func TestRenderExpandedFieldsWithCursor(t *testing.T) {
	entry := gcp.LogEntry{
		ResourceType: "gce_instance",
		InsertID:     "abc",
	}

	// Cursor on first field should highlight it
	result := RenderExpandedFields(entry, 0, 80)
	lines := strings.Split(result, "\n")
	// At least one line should have different styling (highlighted)
	assert.True(t, len(lines) > 0)
}

func TestSeverityAbbreviation(t *testing.T) {
	tests := []struct {
		severity string
		expected string
	}{
		{"INFO", "I"},
		{"WARNING", "W"},
		{"ERROR", "E"},
		{"CRITICAL", "C"},
		{"DEBUG", "D"},
		{"DEFAULT", "D"},
		{"NOTICE", "N"},
		{"ALERT", "A"},
		{"EMERGENCY", "!"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, SeverityAbbrev(tt.severity))
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/ui/components/logviewer/ -run TestRender -v
```

**Step 3: Implement entry rendering**

```go
// internal/ui/components/logviewer/entry.go
package logviewer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

// Severity color mapping
var severityColors = map[string]string{
	"DEBUG":     "#9AA0A6",
	"DEFAULT":   "#9AA0A6",
	"INFO":      "#4285F4",
	"NOTICE":    "#4285F4",
	"WARNING":   "#FBBC04",
	"ERROR":     "#EA4335",
	"CRITICAL":  "#EA4335",
	"ALERT":     "#EA4335",
	"EMERGENCY": "#EA4335",
}

// SeverityAbbrev returns a single-character abbreviation for a severity level.
func SeverityAbbrev(severity string) string {
	switch severity {
	case "INFO":
		return "I"
	case "WARNING":
		return "W"
	case "ERROR":
		return "E"
	case "CRITICAL":
		return "C"
	case "DEBUG", "DEFAULT":
		return "D"
	case "NOTICE":
		return "N"
	case "ALERT":
		return "A"
	case "EMERGENCY":
		return "!"
	default:
		return "?"
	}
}

// severityStyle returns a lipgloss style for the given severity.
func severityStyle(severity string) lipgloss.Style {
	color, ok := severityColors[severity]
	if !ok {
		color = "#9AA0A6"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true)
}

// RenderCompactEntry renders a single log entry in compact (one-line) format.
// expanded controls the collapse/expand indicator.
// selected highlights the line.
func RenderCompactEntry(entry gcp.LogEntry, expanded bool, width int) string {
	indicator := "▸"
	if expanded {
		indicator = "▾"
	}

	sevStyle := severityStyle(entry.Severity)
	abbrev := SeverityAbbrev(entry.Severity)
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")

	resourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8AB4F8"))
	resource := entry.ResourceType
	if resource == "" {
		resource = "unknown"
	}
	// Truncate resource type for display
	if len(resource) > 20 {
		resource = resource[:17] + "..."
	}

	// Calculate remaining width for message
	// indicator(1) + space(1) + severity(1) + space(2) + timestamp(19) + space(2) + resource(var) + space(2) + message
	usedWidth := 1 + 1 + 1 + 2 + 19 + 2 + lipgloss.Width(resource) + 2
	msgWidth := width - usedWidth - 2 // 2 for left padding
	if msgWidth < 10 {
		msgWidth = 10
	}
	message := truncateStr(entry.Message, msgWidth)

	return fmt.Sprintf("  %s %s  %s  %s  %s",
		indicator,
		sevStyle.Render(abbrev),
		timestamp,
		resourceStyle.Render(resource),
		message,
	)
}

// RenderExpandedFields renders the expanded field view for a log entry.
// cursorIdx is the 0-based index of the field the cursor is on (-1 for no cursor).
func RenderExpandedFields(entry gcp.LogEntry, cursorIdx int, width int) string {
	fields := entry.FlattenFields()
	if len(fields) == 0 {
		return ""
	}

	var b strings.Builder
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8AB4F8"))
	valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED"))
	cursorStyle := lipgloss.NewStyle().Background(lipgloss.Color("#3C4043"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368")).Faint(true)

	for i, field := range fields {
		prefix := "      " // 6 spaces indent
		line := fmt.Sprintf("%s: %s", keyStyle.Render(field.Key), valStyle.Render(field.Value))

		if i == cursorIdx {
			// Highlight the selected field and show filter hint
			hint := hintStyle.Render(" [+f]")
			b.WriteString(prefix + cursorStyle.Render(line) + hint)
		} else {
			b.WriteString(prefix + line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// truncateStr truncates a string to maxLen, adding "..." if truncated.
func truncateStr(s string, maxLen int) string {
	// Replace newlines with spaces for compact display
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
```

**Step 4: Run tests**

```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/ui/components/logviewer/ -v
```

**Step 5: Commit**

```bash
git add internal/ui/components/logviewer/entry.go internal/ui/components/logviewer/entry_test.go
git commit -m "2026-03-17: Add log entry compact and expanded rendering"
```

---

## Task 7: LogViewer Component (Model)

**Files:**
- Create: `internal/ui/components/logviewer/logviewer.go`
- Test: `internal/ui/components/logviewer/logviewer_test.go`

**Step 1: Write tests**

```go
// internal/ui/components/logviewer/logviewer_test.go
package logviewer

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func makeTestEntries(n int) []gcp.LogEntry {
	entries := make([]gcp.LogEntry, n)
	for i := range n {
		entries[i] = gcp.LogEntry{
			Timestamp:    time.Now().Add(-time.Duration(n-i) * time.Minute),
			Severity:     "INFO",
			Message:      fmt.Sprintf("message %d", i),
			ResourceType: "gce_instance",
			InsertID:     fmt.Sprintf("id-%d", i),
		}
	}
	return entries
}

func TestLogViewerSetEntries(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	entries := makeTestEntries(5)
	m.SetEntries(entries)

	assert.Equal(t, 5, m.EntryCount())
	assert.Equal(t, 0, m.Cursor())
}

func TestLogViewerExpandCollapse(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(5))

	assert.False(t, m.IsExpanded(0))

	m.ToggleExpand(0)
	assert.True(t, m.IsExpanded(0))

	m.ToggleExpand(0)
	assert.False(t, m.IsExpanded(0))
}

func TestLogViewerExpandAll(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(3))

	m.ExpandAll()
	for i := range 3 {
		assert.True(t, m.IsExpanded(i))
	}

	m.CollapseAll()
	for i := range 3 {
		assert.False(t, m.IsExpanded(i))
	}
}

func TestLogViewerNavigation(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(5))

	assert.Equal(t, 0, m.Cursor())

	m.MoveDown()
	assert.Equal(t, 1, m.Cursor())

	m.MoveDown()
	assert.Equal(t, 2, m.Cursor())

	m.MoveUp()
	assert.Equal(t, 1, m.Cursor())
}

func TestLogViewerNavigationBounds(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(3))

	m.MoveUp() // at top, should stay
	assert.Equal(t, 0, m.Cursor())

	m.MoveDown()
	m.MoveDown()
	m.MoveDown() // past bottom, should stay at 2
	assert.Equal(t, 2, m.Cursor())
}

func TestLogViewerFieldCursor(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	entries := makeTestEntries(1)
	entries[0].ResourceType = "gce_instance"
	entries[0].InsertID = "abc"
	m.SetEntries(entries)

	// Expand the entry
	m.ToggleExpand(0)
	assert.True(t, m.IsExpanded(0))

	// Initially no field selected
	assert.Equal(t, -1, m.FieldCursor())

	// Move into field navigation
	m.EnterFieldNav()
	assert.Equal(t, 0, m.FieldCursor())

	m.FieldDown()
	assert.Equal(t, 1, m.FieldCursor())

	m.ExitFieldNav()
	assert.Equal(t, -1, m.FieldCursor())
}

func TestLogViewerSelectedField(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	entries := []gcp.LogEntry{{
		ResourceType: "gce_instance",
		InsertID:     "abc",
	}}
	m.SetEntries(entries)
	m.ToggleExpand(0)
	m.EnterFieldNav()

	field := m.SelectedField()
	assert.NotNil(t, field)
}

func TestLogViewerNeedMoreMsg(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(20))
	m.SetHasMore(true)

	// Navigate near bottom
	for range 18 {
		m.MoveDown()
	}

	assert.True(t, m.NeedsMore())
}

func TestLogViewerView(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetEntries(makeTestEntries(3))

	view := m.View()
	assert.NotEmpty(t, view)
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/ui/components/logviewer/ -run TestLogViewer -v
```

**Step 3: Implement the LogViewer model**

```go
// internal/ui/components/logviewer/logviewer.go
package logviewer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
)

// NeedMoreLogsMsg is emitted when the viewer needs more log entries (infinite scroll).
type NeedMoreLogsMsg struct{}

// FilterFieldMsg is emitted when the user presses Enter on an expanded field.
type FilterFieldMsg struct {
	Key   string
	Value string
}

// Model is the log entry viewer component.
type Model struct {
	entries  []gcp.LogEntry
	expanded map[int]bool // entry index → expanded
	cursor   int          // selected entry index
	fieldCur int          // selected field within expanded entry (-1 = none)
	hasMore  bool         // more pages available
	width    int
	height   int
	offset   int // scroll offset for virtual scrolling
}

// New creates a new LogViewer model.
func New() *Model {
	return &Model{
		expanded: make(map[int]bool),
		fieldCur: -1,
	}
}

// SetSize sets the available rendering dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetEntries replaces the entry list (e.g., on new query).
func (m *Model) SetEntries(entries []gcp.LogEntry) {
	m.entries = entries
	m.expanded = make(map[int]bool)
	m.cursor = 0
	m.fieldCur = -1
	m.offset = 0
}

// AppendEntries adds entries to the end (for infinite scroll / tail mode).
func (m *Model) AppendEntries(entries []gcp.LogEntry) {
	m.entries = append(m.entries, entries...)
}

// SetHasMore sets whether there are more pages available.
func (m *Model) SetHasMore(hasMore bool) {
	m.hasMore = hasMore
}

// EntryCount returns the number of entries.
func (m *Model) EntryCount() int {
	return len(m.entries)
}

// Cursor returns the current cursor position.
func (m *Model) Cursor() int {
	return m.cursor
}

// FieldCursor returns the field cursor position (-1 = not in field nav).
func (m *Model) FieldCursor() int {
	return m.fieldCur
}

// IsExpanded returns whether an entry at index is expanded.
func (m *Model) IsExpanded(idx int) bool {
	return m.expanded[idx]
}

// ToggleExpand toggles the expand state of an entry.
func (m *Model) ToggleExpand(idx int) {
	if m.expanded[idx] {
		delete(m.expanded, idx)
		m.fieldCur = -1
	} else {
		m.expanded[idx] = true
	}
}

// ExpandAll expands all entries.
func (m *Model) ExpandAll() {
	for i := range m.entries {
		m.expanded[i] = true
	}
}

// CollapseAll collapses all entries.
func (m *Model) CollapseAll() {
	m.expanded = make(map[int]bool)
	m.fieldCur = -1
}

// MoveUp moves the cursor up.
func (m *Model) MoveUp() {
	if m.fieldCur >= 0 {
		// In field navigation mode
		m.FieldUp()
		return
	}
	if m.cursor > 0 {
		m.cursor--
		m.fieldCur = -1
		m.ensureVisible()
	}
}

// MoveDown moves the cursor down.
func (m *Model) MoveDown() {
	if m.fieldCur >= 0 {
		// In field navigation mode
		m.FieldDown()
		return
	}
	if m.cursor < len(m.entries)-1 {
		m.cursor++
		m.fieldCur = -1
		m.ensureVisible()
	}
}

// EnterFieldNav enters field navigation mode for the current expanded entry.
func (m *Model) EnterFieldNav() {
	if !m.expanded[m.cursor] {
		return
	}
	fields := m.entries[m.cursor].FlattenFields()
	if len(fields) > 0 {
		m.fieldCur = 0
	}
}

// ExitFieldNav exits field navigation mode.
func (m *Model) ExitFieldNav() {
	m.fieldCur = -1
}

// FieldUp moves the field cursor up.
func (m *Model) FieldUp() {
	if m.fieldCur > 0 {
		m.fieldCur--
	} else {
		// Exit field nav and go back to entry navigation
		m.fieldCur = -1
	}
}

// FieldDown moves the field cursor down.
func (m *Model) FieldDown() {
	if !m.expanded[m.cursor] {
		return
	}
	fields := m.entries[m.cursor].FlattenFields()
	if m.fieldCur < len(fields)-1 {
		m.fieldCur++
	}
}

// SelectedField returns the currently selected field, or nil.
func (m *Model) SelectedField() *gcp.FlattenedField {
	if m.fieldCur < 0 || !m.expanded[m.cursor] {
		return nil
	}
	fields := m.entries[m.cursor].FlattenFields()
	if m.fieldCur >= len(fields) {
		return nil
	}
	return &fields[m.fieldCur]
}

// NeedsMore returns true if the cursor is near the bottom and more pages are available.
func (m *Model) NeedsMore() bool {
	if !m.hasMore || len(m.entries) == 0 {
		return false
	}
	// Trigger when within 10 entries of the bottom
	return m.cursor >= len(m.entries)-10
}

// ensureVisible adjusts scroll offset so the cursor is visible.
func (m *Model) ensureVisible() {
	// Simple implementation: count visible lines from offset to cursor
	if m.cursor < m.offset {
		m.offset = m.cursor
	}

	// Count lines consumed by entries from offset to cursor
	linesUsed := 0
	for i := m.offset; i <= m.cursor && i < len(m.entries); i++ {
		linesUsed++ // compact line
		if m.expanded[i] {
			linesUsed += len(m.entries[i].FlattenFields())
		}
	}

	// If too many lines, advance offset
	for linesUsed > m.height && m.offset < m.cursor {
		linesUsed-- // remove compact line of offset entry
		if m.expanded[m.offset] {
			linesUsed -= len(m.entries[m.offset].FlattenFields())
		}
		m.offset++
	}
}

// View renders the log viewer.
func (m *Model) View() string {
	if len(m.entries) == 0 {
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		return mutedStyle.Render("  No log entries")
	}

	var b strings.Builder
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("#3C4043"))
	linesRendered := 0

	for i := m.offset; i < len(m.entries) && linesRendered < m.height; i++ {
		entry := m.entries[i]
		isSelected := i == m.cursor
		isExpanded := m.expanded[i]

		// Render compact line
		line := RenderCompactEntry(entry, isExpanded, m.width)
		if isSelected && m.fieldCur < 0 {
			line = selectedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
		linesRendered++

		// Render expanded fields if expanded
		if isExpanded && linesRendered < m.height {
			fieldCurIdx := -1
			if isSelected {
				fieldCurIdx = m.fieldCur
			}
			fieldLines := RenderExpandedFields(entry, fieldCurIdx, m.width)
			fieldCount := strings.Count(fieldLines, "\n")
			b.WriteString(fieldLines)
			linesRendered += fieldCount
		}
	}

	// Loading more indicator
	if m.hasMore && linesRendered < m.height {
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		b.WriteString(mutedStyle.Render("  Loading more..."))
		b.WriteString("\n")
	}

	return b.String()
}
```

**Step 4: Run tests**

```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/ui/components/logviewer/ -v
```

**Step 5: Run linter on the package**

```bash
cd /Users/vlad/dev/my/gcon && make lint 2>&1 | head -30
```

Fix any linter issues.

**Step 6: Commit**

```bash
git add internal/ui/components/logviewer/
git commit -m "2026-03-17: Add LogViewer component with expand/collapse and field navigation"
```

---

## Task 8: Logs Messages

**Files:**
- Create: `internal/ui/views/logs_messages.go`

**Step 1: Define all message types**

```go
// internal/ui/views/logs_messages.go
package views

import "github.com/slayer/gcon/internal/gcp"

// LogsViewRequestMsg requests navigation to the Logs Explorer view.
type LogsViewRequestMsg struct{}

// --- Internal messages for log data loading ---

type logsEntriesLoadedMsg struct {
	entries   []gcp.LogEntry
	nextToken string
	total     int64
}
type logsEntriesErrorMsg struct{ err error }

type logsAppendEntriesMsg struct {
	entries   []gcp.LogEntry
	nextToken string
}
type logsAppendErrorMsg struct{ err error }

type logsHistogramLoadedMsg struct {
	data []gcp.DataPoint
}
type logsHistogramErrorMsg struct{ err error }

type logsResourceTypesLoadedMsg struct {
	types []string
}
type logsResourceTypesErrorMsg struct{ err error }

type logsLogNamesLoadedMsg struct {
	names []string
}
type logsLogNamesErrorMsg struct{ err error }

type logsTailTickMsg struct{}
type logsTailEntriesMsg struct {
	entries []gcp.LogEntry
}
```

**Step 2: Verify compilation**

```bash
cd /Users/vlad/dev/my/gcon && go build ./internal/ui/views/
```

**Step 3: Commit**

```bash
git add internal/ui/views/logs_messages.go
git commit -m "2026-03-17: Add message types for Logs Explorer view"
```

---

## Task 9: LogsView — Core Structure

**Files:**
- Create: `internal/ui/views/logs.go`
- Test: `internal/ui/views/logs_test.go`

This is the largest task. Build the view incrementally.

**Step 1: Create the basic view struct with Init/View/Update/SetContext**

```go
// internal/ui/views/logs.go
package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/logviewer"
	uictx "github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
)

// focusArea tracks which part of the logs view has keyboard focus.
type logsFocusArea int

const (
	logsFocusLogList logsFocusArea = iota
	logsFocusQueryInput
	logsFocusFilterDropdown
)

// logsState tracks the overall view state.
type logsState int

const (
	logsStateIdle    logsState = iota
	logsStateLoading
	logsStateError
)

// filterDropdownType identifies which filter dropdown is open.
type filterDropdownType int

const (
	filterNone filterDropdownType = iota
	filterResources
	filterLogNames
	filterSeverities
)

// LogsView is the main Logs Explorer view.
type LogsView struct {
	projectID string
	gcpClient *gcp.Client

	// State
	state logsState
	err   error

	// Query
	query          string // user's raw LQL input
	queryInput     textinput.Model
	queryFocused   bool

	// Filters
	selectedResources  []string
	selectedLogNames   []string
	selectedSeverities []string
	availableResources []string // lazy-loaded
	availableLogNames  []string // lazy-loaded
	resourcesLoaded    bool
	logNamesLoaded     bool

	// Filter dropdown state
	activeFilter       filterDropdownType
	filterOptions      []string
	filterSelected     map[string]bool
	filterCursor       int
	filterSearch       string

	// Data
	entries       []gcp.LogEntry
	nextPageToken string
	totalCount    int64
	histogramData []gcp.DataPoint
	loadingMore   bool

	// Time range
	timeRange time.Duration

	// Tail mode
	tailMode   bool
	tailTicker *time.Ticker
	tailDone   chan struct{}

	// UI components
	logViewer *logviewer.Model
	spinner   spinner.Model
	focus     logsFocusArea
	width     int
	height    int
	ctx       *uictx.ProgramContext
}

// NewLogsView creates a new Logs Explorer view.
func NewLogsView(projectID string, gcpClient *gcp.Client) *LogsView {
	ti := textinput.New()
	ti.Placeholder = "Enter LQL query (e.g., severity>=WARNING)"
	ti.CharLimit = 1000

	return &LogsView{
		projectID:  projectID,
		gcpClient:  gcpClient,
		timeRange:  time.Hour,
		logViewer:  logviewer.New(),
		spinner:    components.NewGCPSpinner(),
		queryInput: ti,
		focus:      logsFocusLogList,
		filterSelected: make(map[string]bool),
		selectedSeverities: []string{}, // empty = all
	}
}

// Init initializes the view and starts loading logs.
func (v *LogsView) Init() tea.Cmd {
	v.state = logsStateLoading
	v.err = nil
	v.entries = nil
	v.nextPageToken = ""
	return tea.Batch(v.spinner.Tick, v.executeQuery())
}

// SetContext updates the shared program context.
func (v *LogsView) SetContext(ctx *uictx.ProgramContext) {
	v.ctx = ctx
	if ctx != nil {
		v.width = ctx.ContentWidth
		v.height = ctx.ContentHeight
		v.logViewer.SetSize(v.width, v.logListHeight())
	}
}

// HasTextInputFocused returns true when the query input or filter search is active.
func (v *LogsView) HasTextInputFocused() bool {
	return v.queryFocused || v.activeFilter != filterNone
}

// Close cleans up resources (tail mode ticker).
func (v *LogsView) Close() {
	v.stopTail()
}

// logListHeight calculates the available height for the log entry list.
func (v *LogsView) logListHeight() int {
	// Filter bar (1) + query input (1) + sparkline (1) + padding (2)
	overhead := 5
	h := v.height - overhead
	if h < 5 {
		h = 5
	}
	return h
}

// View renders the Logs Explorer.
func (v *LogsView) View() string {
	if v.state == logsStateLoading && len(v.entries) == 0 {
		return renderLoading(v.spinner, "Loading logs...")
	}

	var b strings.Builder

	// Filter bar
	b.WriteString(v.renderFilterBar())
	b.WriteString("\n")

	// Query input
	b.WriteString(v.renderQueryInput())
	b.WriteString("\n")

	// Sparkline
	b.WriteString(logviewer.RenderSparkline(v.histogramData, v.width, v.totalCount))
	b.WriteString("\n")

	// Separator
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368"))
	b.WriteString(sepStyle.Render(strings.Repeat("─", min(v.width-2, 80))))
	b.WriteString("\n")

	// Error display
	if v.err != nil && len(v.entries) == 0 {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		b.WriteString(errorStyle.Render(fmt.Sprintf("  ✗ %s", v.err.Error())))
		b.WriteString("\n")
		mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		b.WriteString(mutedStyle.Render("  Press 'r' to retry"))
		b.WriteString("\n")
		return b.String()
	}

	// Log entries
	v.logViewer.SetSize(v.width, v.logListHeight())
	b.WriteString(v.logViewer.View())

	// Filter dropdown overlay (rendered last to appear on top)
	if v.activeFilter != filterNone {
		// The dropdown is rendered as part of the filter bar above
		// (overlay rendering handled separately)
	}

	return b.String()
}

// renderFilterBar renders the three filter dropdown buttons.
func (v *LogsView) renderFilterBar() string {
	btnStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E8EAED")).
		Background(lipgloss.Color("#3C4043")).
		Padding(0, 1)
	activeStyle := btnStyle.Background(lipgloss.Color("#4285F4"))

	resourceLabel := "All resources"
	if len(v.selectedResources) > 0 {
		resourceLabel = fmt.Sprintf("%d resources", len(v.selectedResources))
	}

	logNameLabel := "All log names"
	if len(v.selectedLogNames) > 0 {
		logNameLabel = fmt.Sprintf("%d log names", len(v.selectedLogNames))
	}

	severityLabel := "All severities"
	if len(v.selectedSeverities) > 0 {
		severityLabel = fmt.Sprintf("%d severities", len(v.selectedSeverities))
	}

	rStyle := btnStyle
	lStyle := btnStyle
	sStyle := btnStyle
	if v.activeFilter == filterResources {
		rStyle = activeStyle
	}
	if v.activeFilter == filterLogNames {
		lStyle = activeStyle
	}
	if v.activeFilter == filterSeverities {
		sStyle = activeStyle
	}

	return fmt.Sprintf("  %s  %s  %s",
		rStyle.Render(resourceLabel+" ▾"),
		lStyle.Render(logNameLabel+" ▾"),
		sStyle.Render(severityLabel+" ▾"),
	)
}

// renderQueryInput renders the LQL query text input.
func (v *LogsView) renderQueryInput() string {
	prefix := "  > "
	if v.queryFocused {
		return prefix + v.queryInput.View()
	}
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	if v.query == "" {
		return prefix + mutedStyle.Render(v.queryInput.Placeholder)
	}
	return prefix + v.query
}
```

**Step 2: Add the Update method**

```go
// Update handles messages for the logs view.
//
//nolint:gocognit,cyclop // View update routing
func (v *LogsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		v.spinner, cmd = v.spinner.Update(msg)
		return cmd

	case logsEntriesLoadedMsg:
		v.state = logsStateIdle
		v.err = nil
		v.entries = msg.entries
		v.nextPageToken = msg.nextToken
		v.totalCount = msg.total
		v.logViewer.SetEntries(msg.entries)
		v.logViewer.SetHasMore(msg.nextToken != "")
		return nil

	case logsEntriesErrorMsg:
		v.state = logsStateError
		v.err = msg.err
		return nil

	case logsAppendEntriesMsg:
		v.loadingMore = false
		v.entries = append(v.entries, msg.entries...)
		v.nextPageToken = msg.nextToken
		v.logViewer.AppendEntries(msg.entries)
		v.logViewer.SetHasMore(msg.nextToken != "")
		return nil

	case logsAppendErrorMsg:
		v.loadingMore = false
		// Don't overwrite main error; just stop loading more
		return nil

	case logsHistogramLoadedMsg:
		v.histogramData = msg.data
		return nil

	case logsHistogramErrorMsg:
		// Non-fatal: sparkline just stays empty
		return nil

	case logsResourceTypesLoadedMsg:
		v.availableResources = msg.types
		v.resourcesLoaded = true
		if v.activeFilter == filterResources {
			v.filterOptions = msg.types
		}
		return nil

	case logsLogNamesLoadedMsg:
		v.availableLogNames = msg.names
		v.logNamesLoaded = true
		if v.activeFilter == filterLogNames {
			v.filterOptions = msg.names
		}
		return nil

	case logsResourceTypesErrorMsg, logsLogNamesErrorMsg:
		// Non-fatal: filter dropdown stays empty
		return nil

	case logsTailTickMsg:
		if !v.tailMode {
			return nil
		}
		return v.fetchTailEntries()

	case logsTailEntriesMsg:
		if len(msg.entries) > 0 {
			v.entries = append(v.entries, msg.entries...)
			v.logViewer.AppendEntries(msg.entries)
			v.totalCount += int64(len(msg.entries))
		}
		return v.scheduleTailTick()

	case tea.KeyMsg:
		return v.handleKeyMsg(msg)
	}

	// Pass through to query input if focused
	if v.queryFocused {
		var cmd tea.Cmd
		v.queryInput, cmd = v.queryInput.Update(msg)
		return cmd
	}

	return nil
}
```

**Step 3: Add key handling**

```go
// handleKeyMsg handles keyboard input for the logs view.
//
//nolint:gocognit,cyclop // Key routing
func (v *LogsView) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	// Filter dropdown active — handle filter keys
	if v.activeFilter != filterNone {
		return v.handleFilterKey(msg)
	}

	// Query input focused — handle input keys
	if v.queryFocused {
		switch msg.String() {
		case "enter":
			v.query = v.queryInput.Value()
			v.queryFocused = false
			v.queryInput.Blur()
			return v.runQuery()
		case "esc":
			v.queryFocused = false
			v.queryInput.Blur()
			return nil
		}
		var cmd tea.Cmd
		v.queryInput, cmd = v.queryInput.Update(msg)
		return cmd
	}

	// Main view keys
	switch msg.String() {
	case "/":
		v.queryFocused = true
		v.queryInput.Focus()
		return v.queryInput.Cursor.BlinkCmd()

	case "j", "down":
		v.logViewer.MoveDown()
		if v.logViewer.NeedsMore() && !v.loadingMore {
			v.loadingMore = true
			return v.loadMoreEntries()
		}
		return nil

	case "k", "up":
		v.logViewer.MoveUp()
		return nil

	case "enter", "right":
		if v.logViewer.FieldCursor() >= 0 {
			// In field navigation — add field as filter
			field := v.logViewer.SelectedField()
			if field != nil {
				return v.addFieldToQuery(field)
			}
			return nil
		}
		idx := v.logViewer.Cursor()
		if v.logViewer.IsExpanded(idx) {
			// Enter field navigation mode
			v.logViewer.EnterFieldNav()
		} else {
			v.logViewer.ToggleExpand(idx)
		}
		return nil

	case "left":
		if v.logViewer.FieldCursor() >= 0 {
			v.logViewer.ExitFieldNav()
			return nil
		}
		idx := v.logViewer.Cursor()
		if v.logViewer.IsExpanded(idx) {
			v.logViewer.ToggleExpand(idx)
		}
		return nil

	case "E":
		v.logViewer.ExpandAll()
		return nil

	case "C":
		v.logViewer.CollapseAll()
		return nil

	// Time range selection
	case "1":
		return v.setTimeRange(components.PredefinedTimeRanges[0].Duration)
	case "2":
		return v.setTimeRange(components.PredefinedTimeRanges[1].Duration)
	case "3":
		return v.setTimeRange(components.PredefinedTimeRanges[2].Duration)
	case "4":
		return v.setTimeRange(components.PredefinedTimeRanges[3].Duration)
	case "5":
		return v.setTimeRange(components.PredefinedTimeRanges[4].Duration)

	// Tail mode
	case "f":
		return v.toggleTailMode()

	// Refresh
	case "r":
		return v.runQuery()

	// Filter dropdowns
	case "R":
		return v.openFilterDropdown(filterResources)
	case "L":
		return v.openFilterDropdown(filterLogNames)
	case "V":
		return v.openFilterDropdown(filterSeverities)
	}

	return nil
}
```

**Step 4: Add helper methods**

```go
// buildEffectiveQuery combines filter selections with the user's manual LQL.
func (v *LogsView) buildEffectiveQuery() string {
	var parts []string

	// Time range filter
	cutoff := time.Now().Add(-v.timeRange).UTC().Format(time.RFC3339)
	parts = append(parts, fmt.Sprintf(`timestamp >= "%s"`, cutoff))

	// Resource type filter
	if len(v.selectedResources) > 0 {
		if len(v.selectedResources) == 1 {
			parts = append(parts, fmt.Sprintf(`resource.type = "%s"`, v.selectedResources[0]))
		} else {
			quoted := make([]string, len(v.selectedResources))
			for i, r := range v.selectedResources {
				quoted[i] = fmt.Sprintf(`"%s"`, r)
			}
			parts = append(parts, fmt.Sprintf("resource.type = (%s)", strings.Join(quoted, " OR ")))
		}
	}

	// Log name filter
	if len(v.selectedLogNames) > 0 {
		if len(v.selectedLogNames) == 1 {
			parts = append(parts, fmt.Sprintf(`logName = "%s"`, v.selectedLogNames[0]))
		} else {
			quoted := make([]string, len(v.selectedLogNames))
			for i, n := range v.selectedLogNames {
				quoted[i] = fmt.Sprintf(`"%s"`, n)
			}
			parts = append(parts, fmt.Sprintf("logName = (%s)", strings.Join(quoted, " OR ")))
		}
	}

	// Severity filter
	if len(v.selectedSeverities) > 0 {
		clauses := severityORClauses(v.selectedSeverities)
		parts = append(parts, "("+strings.Join(clauses, " OR ")+")")
	}

	// User query
	if v.query != "" {
		parts = append(parts, v.query)
	}

	return strings.Join(parts, "\nAND ")
}

// severityORClauses builds severity="X" filter clauses.
func severityORClauses(severities []string) []string {
	clauses := make([]string, len(severities))
	for i, s := range severities {
		clauses[i] = fmt.Sprintf(`severity="%s"`, s)
	}
	return clauses
}

// executeQuery runs the current query and fetches histogram data in parallel.
func (v *LogsView) executeQuery() tea.Cmd {
	return tea.Batch(v.fetchEntries(), v.fetchHistogram())
}

// runQuery resets state and executes the query.
func (v *LogsView) runQuery() tea.Cmd {
	v.state = logsStateLoading
	v.err = nil
	v.entries = nil
	v.nextPageToken = ""
	v.logViewer.SetEntries(nil)
	return v.executeQuery()
}

// fetchEntries fetches the first page of log entries.
func (v *LogsView) fetchEntries() tea.Cmd {
	projectID := v.projectID
	client := v.gcpClient
	filter := v.buildEffectiveQuery()

	return func() tea.Msg {
		if client == nil {
			return logsEntriesErrorMsg{err: uierrors.ErrGCPClientNotInitialized}
		}

		logClient, err := client.GetLoggingClient(projectID)
		if err != nil {
			return logsEntriesErrorMsg{err: fmt.Errorf("logging client: %w", err)}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		entries, nextToken, err := logClient.ListLogEntries(ctx, filter, 100, "")
		if err != nil {
			return logsEntriesErrorMsg{err: err}
		}

		return logsEntriesLoadedMsg{
			entries:   entries,
			nextToken: nextToken,
			total:     int64(len(entries)), // approximate; refine with histogram
		}
	}
}

// loadMoreEntries fetches the next page (infinite scroll).
func (v *LogsView) loadMoreEntries() tea.Cmd {
	projectID := v.projectID
	client := v.gcpClient
	filter := v.buildEffectiveQuery()
	pageToken := v.nextPageToken

	return func() tea.Msg {
		if client == nil {
			return logsAppendErrorMsg{err: uierrors.ErrGCPClientNotInitialized}
		}

		logClient, err := client.GetLoggingClient(projectID)
		if err != nil {
			return logsAppendErrorMsg{err: fmt.Errorf("logging client: %w", err)}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		entries, nextToken, err := logClient.ListLogEntries(ctx, filter, 100, pageToken)
		if err != nil {
			return logsAppendErrorMsg{err: err}
		}

		return logsAppendEntriesMsg{
			entries:   entries,
			nextToken: nextToken,
		}
	}
}

// fetchHistogram fetches log entry count data for sparkline.
func (v *LogsView) fetchHistogram() tea.Cmd {
	projectID := v.projectID
	client := v.gcpClient
	timeRange := v.timeRange

	return func() tea.Msg {
		if client == nil {
			return logsHistogramErrorMsg{err: uierrors.ErrGCPClientNotInitialized}
		}

		monClient, err := client.GetMonitoringClient(projectID)
		if err != nil {
			return logsHistogramErrorMsg{err: fmt.Errorf("monitoring client: %w", err)}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		data, err := monClient.GetLogEntryCount(ctx, timeRange)
		if err != nil {
			return logsHistogramErrorMsg{err: err}
		}

		return logsHistogramLoadedMsg{data: data}
	}
}

// setTimeRange changes the time range and re-runs the query.
func (v *LogsView) setTimeRange(d time.Duration) tea.Cmd {
	if v.timeRange == d {
		return nil
	}
	v.timeRange = d
	return v.runQuery()
}

// addFieldToQuery appends a field filter to the query input.
func (v *LogsView) addFieldToQuery(field *gcp.FlattenedField) tea.Cmd {
	clause := fmt.Sprintf(`%s="%s"`, field.Key, field.Value)
	if v.query != "" {
		v.query += "\nAND " + clause
	} else {
		v.query = clause
	}
	v.queryInput.SetValue(v.query)
	return v.runQuery()
}

// --- Tail Mode ---

func (v *LogsView) toggleTailMode() tea.Cmd {
	v.tailMode = !v.tailMode
	if v.tailMode {
		return v.startTail()
	}
	v.stopTail()
	return nil
}

func (v *LogsView) startTail() tea.Cmd {
	v.stopTail() // clean up previous
	v.tailTicker = time.NewTicker(5 * time.Second)
	v.tailDone = make(chan struct{})
	return v.scheduleTailTick()
}

func (v *LogsView) stopTail() {
	v.tailMode = false
	if v.tailTicker != nil {
		v.tailTicker.Stop()
		v.tailTicker = nil
	}
	if v.tailDone != nil {
		close(v.tailDone)
		v.tailDone = nil
	}
}

func (v *LogsView) scheduleTailTick() tea.Cmd {
	ticker := v.tailTicker
	done := v.tailDone
	if ticker == nil || done == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case <-ticker.C:
			return logsTailTickMsg{}
		case <-done:
			return nil
		}
	}
}

func (v *LogsView) fetchTailEntries() tea.Cmd {
	projectID := v.projectID
	client := v.gcpClient
	// Build filter for entries newer than the last one
	var sinceFilter string
	if len(v.entries) > 0 {
		// entries are newest-first, so first entry is the most recent
		lastTimestamp := v.entries[0].Timestamp.UTC().Format(time.RFC3339Nano)
		sinceFilter = fmt.Sprintf(`timestamp > "%s"`, lastTimestamp)
	}

	baseFilter := v.buildEffectiveQuery()
	filter := baseFilter
	if sinceFilter != "" {
		// Replace the timestamp >= clause with timestamp > lastEntry
		filter = sinceFilter
		// Re-add non-time filters
		if v.query != "" {
			filter += "\nAND " + v.query
		}
	}

	return func() tea.Msg {
		if client == nil {
			return logsTailEntriesMsg{} // no-op
		}

		logClient, err := client.GetLoggingClient(projectID)
		if err != nil {
			return logsTailEntriesMsg{}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		entries, _, err := logClient.ListLogEntries(ctx, filter, 50, "")
		if err != nil {
			return logsTailEntriesMsg{}
		}

		return logsTailEntriesMsg{entries: entries}
	}
}

// --- Filter Dropdowns ---

func (v *LogsView) openFilterDropdown(filterType filterDropdownType) tea.Cmd {
	v.activeFilter = filterType
	v.filterCursor = 0
	v.filterSearch = ""
	v.filterSelected = make(map[string]bool)

	switch filterType {
	case filterResources:
		// Copy current selections
		for _, r := range v.selectedResources {
			v.filterSelected[r] = true
		}
		if v.resourcesLoaded {
			v.filterOptions = v.availableResources
			return nil
		}
		return v.loadResourceTypes()

	case filterLogNames:
		for _, n := range v.selectedLogNames {
			v.filterSelected[n] = true
		}
		if v.logNamesLoaded {
			v.filterOptions = v.availableLogNames
			return nil
		}
		return v.loadLogNames()

	case filterSeverities:
		for _, s := range v.selectedSeverities {
			v.filterSelected[s] = true
		}
		v.filterOptions = []string{
			"DEFAULT", "DEBUG", "INFO", "NOTICE",
			"WARNING", "ERROR", "CRITICAL", "ALERT", "EMERGENCY",
		}
		return nil
	}

	return nil
}

func (v *LogsView) handleFilterKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "j", "down":
		if v.filterCursor < len(v.filterOptions)-1 {
			v.filterCursor++
		}
		return nil

	case "k", "up":
		if v.filterCursor > 0 {
			v.filterCursor--
		}
		return nil

	case " ":
		// Toggle selection
		if v.filterCursor < len(v.filterOptions) {
			opt := v.filterOptions[v.filterCursor]
			if v.filterSelected[opt] {
				delete(v.filterSelected, opt)
			} else {
				v.filterSelected[opt] = true
			}
		}
		return nil

	case "enter":
		// Apply selections
		v.applyFilterSelections()
		v.activeFilter = filterNone
		return v.runQuery()

	case "esc":
		v.activeFilter = filterNone
		return nil
	}

	return nil
}

func (v *LogsView) applyFilterSelections() {
	var selected []string
	for opt, sel := range v.filterSelected {
		if sel {
			selected = append(selected, opt)
		}
	}

	switch v.activeFilter {
	case filterResources:
		v.selectedResources = selected
	case filterLogNames:
		v.selectedLogNames = selected
	case filterSeverities:
		v.selectedSeverities = selected
	}
}

func (v *LogsView) loadResourceTypes() tea.Cmd {
	projectID := v.projectID
	client := v.gcpClient

	return func() tea.Msg {
		if client == nil {
			return logsResourceTypesErrorMsg{err: uierrors.ErrGCPClientNotInitialized}
		}

		logClient, err := client.GetLoggingClient(projectID)
		if err != nil {
			return logsResourceTypesErrorMsg{err: err}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		types, err := logClient.ListResourceTypes(ctx)
		if err != nil {
			return logsResourceTypesErrorMsg{err: err}
		}

		return logsResourceTypesLoadedMsg{types: types}
	}
}

func (v *LogsView) loadLogNames() tea.Cmd {
	projectID := v.projectID
	client := v.gcpClient

	return func() tea.Msg {
		if client == nil {
			return logsLogNamesErrorMsg{err: uierrors.ErrGCPClientNotInitialized}
		}

		logClient, err := client.GetLoggingClient(projectID)
		if err != nil {
			return logsLogNamesErrorMsg{err: err}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		names, err := logClient.ListLogNames(ctx)
		if err != nil {
			return logsLogNamesErrorMsg{err: err}
		}

		return logsLogNamesLoadedMsg{names: names}
	}
}
```

**Step 5: Write basic tests**

```go
// internal/ui/views/logs_test.go
package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogsViewNew(t *testing.T) {
	v := NewLogsView("test-project", nil)
	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.Equal(t, logsFocusLogList, v.focus)
}

func TestLogsViewHasTextInputFocused(t *testing.T) {
	v := NewLogsView("test-project", nil)

	assert.False(t, v.HasTextInputFocused())

	v.queryFocused = true
	assert.True(t, v.HasTextInputFocused())

	v.queryFocused = false
	v.activeFilter = filterResources
	assert.True(t, v.HasTextInputFocused())
}

func TestLogsViewBuildEffectiveQuery(t *testing.T) {
	v := NewLogsView("test-project", nil)

	// Default: only time range
	q := v.buildEffectiveQuery()
	assert.Contains(t, q, "timestamp >=")

	// With user query
	v.query = `severity>=WARNING`
	q = v.buildEffectiveQuery()
	assert.Contains(t, q, "severity>=WARNING")
	assert.Contains(t, q, "timestamp >=")

	// With resource filter
	v.selectedResources = []string{"gce_instance"}
	q = v.buildEffectiveQuery()
	assert.Contains(t, q, `resource.type = "gce_instance"`)

	// With multiple resources
	v.selectedResources = []string{"gce_instance", "cloud_run_revision"}
	q = v.buildEffectiveQuery()
	assert.Contains(t, q, "resource.type = (")
	assert.Contains(t, q, "OR")
}

func TestLogsViewFilterDropdown(t *testing.T) {
	v := NewLogsView("test-project", nil)

	// Open severity filter (static, no API call needed)
	v.openFilterDropdown(filterSeverities)
	assert.Equal(t, filterSeverities, v.activeFilter)
	assert.True(t, len(v.filterOptions) > 0)
	assert.Contains(t, v.filterOptions, "ERROR")
}
```

**Step 6: Verify compilation and run tests**

```bash
cd /Users/vlad/dev/my/gcon && go build ./internal/ui/views/ && go test ./internal/ui/views/ -run TestLogsView -v
```

**Step 7: Commit**

```bash
git add internal/ui/views/logs.go internal/ui/views/logs_test.go
git commit -m "2026-03-17: Add LogsView core structure with query building and filter dropdowns"
```

---

## Task 10: App Integration — Sidebar and Command Palette

**Files:**
- Modify: `internal/ui/components/sidebar/menu.go`
- Modify: `internal/ui/components/commandpalette/commands.go`

**Step 1: Add ViewLogs to sidebar menu.go**

Add to the `ViewType` constants (after `ViewCloudRunServices`):

```go
ViewLogs
```

Add icon constants:

```go
IconLogging     = "◈" // Logging category
IconLogsExplorer = "◆" // Logs Explorer leaf
```

Add to `DefaultMenu()` — new category after Cloud Run:

```go
{
    ID:     "logging",
    Label:  "Logging",
    Icon:   IconLogging,
    Hotkey: 'G',
    Type:   MenuItemCategory,
    Children: []MenuItem{
        {ID: "logs-explorer", Label: "Logs Explorer", Icon: IconLogsExplorer, Hotkey: 'g', Type: MenuItemLeaf, ViewType: ViewLogs},
    },
},
```

**Step 2: Add ViewLogs to commandpalette commands.go**

Add to the `ViewType` constants (after `ViewCloudRunServices`):

```go
ViewLogs
```

Add icon:

```go
IconLogs = "◆"
```

Add to `NavigationCommands()`:

```go
{
    ID:       "nav:logs-explorer",
    Label:    "Logging: Logs Explorer",
    Icon:     IconLogs,
    Type:     CommandTypeNavigation,
    ViewType: ViewLogs,
    Enabled:  true,
},
```

**Step 3: Verify compilation**

```bash
cd /Users/vlad/dev/my/gcon && go build ./...
```

**Step 4: Commit**

```bash
git add internal/ui/components/sidebar/menu.go internal/ui/components/commandpalette/commands.go
git commit -m "2026-03-17: Add Logging section to sidebar and command palette"
```

---

## Task 11: App Integration — View Lifecycle

**Files:**
- Modify: `internal/ui/app.go` (App struct, getCurrentViewModel, updateViewSizes)
- Modify: `internal/ui/app_render.go` (renderCurrentView, renderHeader)
- Modify: `internal/ui/app_navigation.go` (handleLogsRequest, clearAllViews, updateSidebarActiveView, sidebar guards)

**Step 1: Add logsView field to App struct**

In `internal/ui/app.go`, add after `instanceConfigEditView`:

```go
logsView *views.LogsView
```

**Step 2: Add getCurrentViewModel case**

In `getCurrentViewModel()`, add before the `ViewFormDemo` case:

```go
case ViewLogs:
    return a.logsView
```

**Step 3: Add updateViewSizes case**

In `updateViewSizes()`, add:

```go
if a.logsView != nil {
    a.logsView.SetContext(a.ctx)
}
```

**Step 4: Add renderCurrentView case**

In `internal/ui/app_render.go`, `renderCurrentView()`, add:

```go
case ViewLogs:
    if a.logsView != nil {
        return a.logsView.View()
    }
```

**Step 5: Add renderHeader category and breadcrumbs**

In `renderHeader()`, add to the category switch:

```go
case ViewLogs:
    category = "Logging"
```

Add to the breadcrumb resources switch:

```go
case ViewLogs:
    resources = append(resources, "Logs Explorer")
```

**Step 6: Add navigation handler in app_navigation.go**

Add `handleLogsRequest`:

```go
func (a *App) handleLogsRequest() tea.Cmd {
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewLogs
	a.logsView = views.NewLogsView(a.selectedProject.ID, a.gcpClient)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.logsView.Init()
}
```

**Step 7: Add to clearAllViews**

```go
if a.logsView != nil {
    a.logsView.Close()
}
a.logsView = nil
```

**Step 8: Add to updateSidebarActiveView**

```go
case ViewLogs:
    a.sidebar.SetActiveView(sidebar.ViewLogs)
```

**Step 9: Add sidebar navigation guard**

In the sidebar navigation handler (`handleSidebarNavigation` or wherever sidebar item clicks are processed), add:

```go
case sidebar.ViewLogs:
    if a.currentView != ViewLogs {
        a.logsView = views.NewLogsView(projectID, a.gcpClient)
        a.currentView = ViewLogs
        a.updateSidebarActiveView()
        a.updateViewSizes()
        return a.logsView.Init()
    }
```

**Step 10: Add message handlers in app.go Update()**

Add cases in the Update() message switch:

```go
case views.LogsViewRequestMsg:
    return a, a.handleLogsRequest()
```

**Step 11: Add back navigation cleanup**

In the Esc/back handler, add:

```go
case ViewLogs:
    if a.logsView != nil {
        a.logsView.Close()
    }
    a.logsView = nil
```

**Step 12: Verify compilation**

```bash
cd /Users/vlad/dev/my/gcon && go build ./...
```

**Step 13: Run full test suite**

```bash
cd /Users/vlad/dev/my/gcon && go test ./... 2>&1 | tail -20
```

**Step 14: Commit**

```bash
git add internal/ui/app.go internal/ui/app_render.go internal/ui/app_navigation.go
git commit -m "2026-03-17: Integrate LogsView into app lifecycle (navigation, rendering, cleanup)"
```

---

## Task 12: Final Polish and Lint

**Files:**
- Various lint fixes
- Modify: `CLAUDE.md` — update implemented features
- Modify: Key bindings doc

**Step 1: Run linter**

```bash
cd /Users/vlad/dev/my/gcon && make lint 2>&1
```

Fix all linter issues reported.

**Step 2: Run full test suite**

```bash
cd /Users/vlad/dev/my/gcon && make test
```

**Step 3: Update CLAUDE.md implemented features**

Add to the "Implemented Features" section:

```markdown
- [x] Cloud Logging Explorer
  - LQL query input with auto-resize
  - Quick filters (Resources, Log Names, Severities)
  - Sparkline histogram for log density
  - Expandable log entries with field-level cursor
  - Filter-by-field (Enter on expanded field appends to query)
  - Infinite scroll pagination
  - Tail mode (live streaming, 5s polling)
  - Time range selection (1h/6h/24h/7d/30d)
  - Severity color coding
```

Remove from "Planned Features":
```
- [ ] Cloud Logging viewer with filters
```

**Step 4: Update key bindings in CLAUDE.md and key-bindings rule**

Add a new "Logs Explorer" section to `.claude/rules/key-bindings.md`:

```markdown
## Logs Explorer

| Key | Action |
|-----|--------|
| `/` | Focus query input |
| `Enter` | Run query (input) / Expand entry / Filter by field |
| `Shift+Enter` | Newline in query |
| `Esc` | Blur input / Collapse / Close filter / Go back |
| `j/k` or `↓/↑` | Navigate entries |
| `→` or `Enter` | Expand / Enter field nav |
| `←` | Collapse / Exit field nav |
| `E` | Expand all entries |
| `C` | Collapse all entries |
| `1-5` | Time range (1h/6h/24h/7d/30d) |
| `f` | Toggle tail mode |
| `r` | Refresh (re-run query) |
| `R` | Open resource filter |
| `L` | Open log name filter |
| `V` | Open severity filter |
```

**Step 5: Run linter again**

```bash
cd /Users/vlad/dev/my/gcon && make lint
```

**Step 6: Commit**

```bash
git add -A
git commit -m "2026-03-17: Polish logging explorer - lint fixes, docs, key bindings"
```

---

## Key Technical Notes for the Implementer

### Imports to watch for

- `cloud.google.com/go/logging` — used by `convertLogEntry` (for `logging.Entry` type)
- `google.golang.org/genproto/googleapis/api/monitoredres` — for `MonitoredResource` in tests
- Run `go mod tidy` after adding new imports

### Patterns to follow

- **Spinner**: Always `components.NewGCPSpinner()`, never inline `spinner.New()`
- **Tail mode**: Use `time.Ticker` + `done chan struct{}` (NOT `tea.Tick`) — see `cloudrun_observability.go`
- **Error colors**: `#EA4335` for errors, `#FBBC04` for warnings, `#4285F4` for info
- **Loading state**: Use `renderLoading(v.spinner, "Loading logs...")` from `helpers.go`

### Testing approach

- Unit test the `FlattenFields`, sparkline, and entry rendering in isolation
- Test the `LogsView` key handling and query building without a GCP client (pass nil)
- Integration testing requires real GCP credentials (skip in CI)

### Common pitfalls

1. `logadmin.Entries()` returns newest-first — entries are already in the right order for display
2. `fetchMetricData` sorts ascending — sparkline data is oldest-first
3. The `Close()` method MUST be called in `clearAllViews()` to stop the tail ticker
4. `HasTextInputFocused()` MUST return true when query input OR filter dropdown is active
