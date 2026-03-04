package gcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/logging/logadmin"
	"google.golang.org/api/iterator"
)

// LoggingClient provides access to Cloud Logging
type LoggingClient struct {
	adminClient *logadmin.Client
	projectID   string
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp      time.Time
	Severity       string // INFO, WARNING, ERROR, CRITICAL
	Message        string
	SourceLocation string
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
func (c *LoggingClient) GetCloudRunLogs(
	ctx context.Context,
	serviceName string,
	severities []string,
	limit int,
) ([]LogEntry, error) {
	// Filter by Cloud Run revision resource with the given service name
	filter := fmt.Sprintf(`resource.type="cloud_run_revision" AND resource.labels.service_name="%s"`, serviceName) //nolint:gocritic // GCP filter syntax requires double quotes

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

// severityORClauses builds severity="X" filter clauses for each provided severity level.
func severityORClauses(severities []string) []string {
	clauses := make([]string, len(severities))
	for i, s := range severities {
		clauses[i] = fmt.Sprintf(`severity="%s"`, s) //nolint:gocritic // GCP filter syntax requires double quotes
	}
	return clauses
}
