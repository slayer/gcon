package gcp

import (
	"context"
	"errors"
	"fmt"
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
			Severity:  entry.Severity.String(),
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
