package views

import (
	"time"

	"github.com/slayer/gcon/internal/gcp"
)

// LogsViewRequestMsg requests navigation to the Logs Explorer view.
// Optional fields allow pre-populating filters (e.g., from Cloud Run observability).
type LogsViewRequestMsg struct {
	// Query is an LQL filter string pre-filled in the query input.
	Query string
	// Severities pre-selects severity filters (e.g., []string{"ERROR", "WARNING"}).
	Severities []string
	// TimeRange pre-sets the time window. Zero means use default (1h).
	TimeRange time.Duration
}

// --- Internal messages for log data loading ---

type logsEntriesLoadedMsg struct {
	entries   []gcp.LogEntry
	nextToken string
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

type logsExportDoneMsg struct {
	message string // success or error message
}
