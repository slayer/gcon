package gcp

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/logging/logadmin"
	"google.golang.org/api/iterator"
)

// LoggingClient provides access to Cloud Logging
type LoggingClient struct {
	adminClient *logadmin.Client
	projectID   string
}

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
// Useful for rendering a detail view of a log entry where nested maps are expanded.
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
	// Recursively flatten JSON payload into dot-notated keys
	flattenMap("jsonPayload", e.JSONPayload, &fields)

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Key < fields[j].Key
	})

	return fields
}

// flattenMap recursively flattens a nested map into dot-notated key-value pairs.
func flattenMap(prefix string, m map[string]any, out *[]FlattenedField) {
	if m == nil {
		return
	}

	// Sort keys for deterministic output within each level
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fullKey := prefix + "." + k
		switch v := m[k].(type) {
		case map[string]any:
			flattenMap(fullKey, v, out)
		default:
			*out = append(*out, FlattenedField{Key: fullKey, Value: fmt.Sprintf("%v", v)})
		}
	}
}

// NewLoggingClient creates a new Cloud Logging client
func NewLoggingClient(ctx context.Context, projectID string) (*LoggingClient, error) {
	client, err := logadmin.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create logging client: %w", err)
	}

	return &LoggingClient{
		adminClient: client,
		projectID:   projectID,
	}, nil
}

// Close closes the logging client
func (c *LoggingClient) Close() error {
	return c.adminClient.Close()
}

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

	if entry.Resource != nil {
		logEntry.ResourceType = entry.Resource.Type
		logEntry.ResourceLabels = entry.Resource.Labels
	}

	logEntry.Labels = entry.Labels

	if entry.SourceLocation != nil {
		logEntry.SourceLocation = fmt.Sprintf("%s:%d", entry.SourceLocation.File, entry.SourceLocation.Line)
	}

	// Extract message from payload, preserving structured data when available
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
// pageToken format: "RFC3339Nano_timestamp|insertID" for cursor-based pagination.
func (c *LoggingClient) ListLogEntries(
	ctx context.Context,
	filter string,
	pageSize int,
	pageToken string,
) ([]LogEntry, string, error) {
	effectiveFilter := filter

	// Apply cursor-based pagination by narrowing the timestamp window
	// and skipping the last entry from the previous page via insertID.
	skipInsertID := ""
	if pageToken != "" {
		parts := strings.SplitN(pageToken, "|", 2)
		if len(parts) == 2 {
			effectiveFilter += fmt.Sprintf(` AND timestamp <= %q`, parts[0])
			skipInsertID = parts[1]
		}
	}

	iter := c.adminClient.Entries(ctx,
		logadmin.Filter(effectiveFilter),
		logadmin.NewestFirst(),
	)

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

		// Skip the boundary entry from the previous page to avoid duplicates
		if skipInsertID != "" && converted.InsertID == skipInsertID {
			skipInsertID = ""
			continue
		}

		entries = append(entries, converted)
		count++
	}

	// Build next page token from the last entry so the caller can fetch more
	nextToken := ""
	if count == pageSize && len(entries) > 0 {
		last := entries[len(entries)-1]
		nextToken = last.Timestamp.UTC().Format(time.RFC3339Nano) + "|" + last.InsertID
	}

	return entries, nextToken, nil
}

// GetRecentLogs fetches recent logs for an instance
func (c *LoggingClient) GetRecentLogs(
	ctx context.Context,
	instanceID string,
	zone string,
	severity string,
	limit int,
) ([]LogEntry, error) {
	// Build filter for GCE instance logs
	filter := fmt.Sprintf(`resource.type="gce_instance"
		AND resource.labels.instance_id="%s"
		AND resource.labels.zone="%s"
		AND severity>=%s`, instanceID, zone, severity)

	// Query logs
	iter := c.adminClient.Entries(ctx,
		logadmin.Filter(filter),
		logadmin.NewestFirst(),
	)

	var entries []LogEntry
	count := 0

	for count < limit {
		entry, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to fetch log entries: %w", err)
		}

		logEntry := LogEntry{
			Timestamp: entry.Timestamp,
			Severity:  normalizeSeverity(entry.Severity.String()),
			Message:   fmt.Sprintf("%v", entry.Payload),
		}

		// Extract source location if available
		if entry.SourceLocation != nil {
			logEntry.SourceLocation = fmt.Sprintf("%s:%d",
				entry.SourceLocation.File,
				entry.SourceLocation.Line)
		}

		entries = append(entries, logEntry)
		count++
	}

	return entries, nil
}

// GetCloudRunLogs fetches recent logs for a Cloud Run service.
// severities filters by log severity (e.g. ["INFO", "WARNING"]). nil or empty means all severities.
// duration limits how far back to look (0 means no time filter).
func (c *LoggingClient) GetCloudRunLogs(
	ctx context.Context,
	serviceName string,
	severities []string,
	duration time.Duration,
	limit int,
) ([]LogEntry, error) {
	// Filter by Cloud Run revision resource with the given service name
	filter := fmt.Sprintf(`resource.type="cloud_run_revision" AND resource.labels.service_name="%s"`, serviceName) //nolint:gocritic // GCP filter syntax requires double quotes

	// Apply time range filter
	if duration > 0 {
		cutoff := time.Now().Add(-duration).UTC().Format(time.RFC3339)
		filter += fmt.Sprintf(` AND timestamp >= "%s"`, cutoff) //nolint:gocritic // GCP filter syntax requires double quotes
	}

	// Optionally restrict to specific severity levels
	if len(severities) > 0 {
		clauses := severityORClauses(severities)
		filter += " AND (" + strings.Join(clauses, " OR ") + ")"
	}

	iter := c.adminClient.Entries(ctx,
		logadmin.Filter(filter),
		logadmin.NewestFirst(),
	)

	var entries []LogEntry
	count := 0

	for count < limit {
		entry, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Cloud Run log entries: %w", err)
		}

		logEntry := LogEntry{
			Timestamp: entry.Timestamp,
			Severity:  normalizeSeverity(entry.Severity.String()),
			Message:   fmt.Sprintf("%v", entry.Payload),
		}

		if entry.SourceLocation != nil {
			logEntry.SourceLocation = fmt.Sprintf("%s:%d",
				entry.SourceLocation.File,
				entry.SourceLocation.Line)
		}

		entries = append(entries, logEntry)
		count++
	}

	return entries, nil
}

// normalizeSeverity converts Go logging library severity strings ("Default", "Info", etc.)
// to uppercase form ("INFO", "WARNING", etc.) matching GCP console conventions.
// "Default" maps to "INFO" since it represents stdout/stderr without explicit severity.
func normalizeSeverity(sev string) string {
	upper := strings.ToUpper(sev)
	if upper == "DEFAULT" {
		return "INFO"
	}
	return upper
}

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

// severityORClauses builds severity="X" filter clauses for each provided severity level.
func severityORClauses(severities []string) []string {
	clauses := make([]string, len(severities))
	for i, s := range severities {
		clauses[i] = fmt.Sprintf(`severity="%s"`, s) //nolint:gocritic // GCP filter syntax requires double quotes
	}
	return clauses
}
