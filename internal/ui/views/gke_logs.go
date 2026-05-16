package views

import (
	gocontext "context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
)

// gkeLogs is the Logs sub-view: an embedded log viewer with severity
// toggles, a resource-type dropdown, and 15 s opt-in auto-refresh.
//
// Unlike the dedicated Logs Explorer view, this sub-view ships with a
// fixed cluster-scope filter (resource.type + cluster_name + location +
// severity). The `L` key (Task 11) hands the same LQL off to the full
// explorer for advanced editing.
type gkeLogs struct {
	projectID, location, clusterName string
	gcpClient                        *gcp.Client

	infoOn, warnOn, errOn bool
	resourceType          string // "k8s_cluster" by default
	autoRefresh           bool
	tabActive             bool

	entries []gcp.LogEntry
	err     error
	loading bool

	width int
}

func newGKELogs(projectID, location, clusterName string, client *gcp.Client) *gkeLogs {
	return &gkeLogs{
		projectID:    projectID,
		location:     location,
		clusterName:  clusterName,
		gcpClient:    client,
		infoOn:       true,
		warnOn:       true,
		errOn:        true,
		resourceType: "k8s_cluster",
		autoRefresh:  false,
	}
}

// SetTabActive marks the sub-view active/inactive so pending auto-refresh
// ticks can be ignored after the user leaves the tab.
func (s *gkeLogs) SetTabActive(active bool) { s.tabActive = active }

func (s *gkeLogs) Init() tea.Cmd {
	s.loading = true
	return tea.Batch(components.NewGCPSpinner().Tick, s.fetchLogs(), s.tickAutoRefresh())
}

// Refresh re-runs the current filter without touching toggle state.
func (s *gkeLogs) Refresh() tea.Cmd {
	s.loading = true
	s.err = nil
	return s.fetchLogs()
}

func (s *gkeLogs) fetchLogs() tea.Cmd {
	if s.gcpClient == nil {
		return nil
	}
	query := s.buildLQL()
	projectID := s.projectID
	return func() tea.Msg {
		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 30*time.Second)
		defer cancel()
		lc, err := s.gcpClient.GetLoggingClient(projectID)
		if err != nil {
			return gkeLogsErrorMsg{err: fmt.Errorf("logging client: %w", err)}
		}
		// ListLogEntries returns (entries, nextPageToken, error). Phase 2a
		// fetches the first page only; pagination wiring lands later.
		entries, _, err := lc.ListLogEntries(ctx, query, 100, "")
		if err != nil {
			return gkeLogsErrorMsg{err: err}
		}
		return gkeLogsLoadedMsg{entries: entries}
	}
}

// buildLQL composes the Cloud Logging filter from the sub-view's toggle state.
// Severity uses the lowest enabled level so the LQL stays simple — client-side
// filtering (Task 11) hides individual rows when only a higher level is on.
func (s *gkeLogs) buildLQL() string {
	parts := []string{
		fmt.Sprintf(`resource.type = "%s"`, s.resourceType),
		fmt.Sprintf(`resource.labels.cluster_name = "%s"`, s.clusterName),
		fmt.Sprintf(`resource.labels.location = "%s"`, s.location),
	}
	switch {
	case s.infoOn:
		parts = append(parts, `severity >= INFO`)
	case s.warnOn:
		parts = append(parts, `severity >= WARNING`)
	case s.errOn:
		parts = append(parts, `severity >= ERROR`)
	default:
		// All toggles off — fall back to all severities; the View layer
		// renders an empty body when no toggles are on.
		parts = append(parts, `severity >= INFO`)
	}
	return strings.Join(parts, " AND ")
}

// tickAutoRefresh schedules a single auto-refresh tick. Each tick reschedules
// the next one (see cloudrun_observability.go for the same pattern). tea.Tick
// messages survive context switches, so the handler must double-check
// (autoRefresh && tabActive) on receipt.
func (s *gkeLogs) tickAutoRefresh() tea.Cmd {
	if !s.autoRefresh || !s.tabActive {
		return nil
	}
	return tea.Tick(15*time.Second, func(_ time.Time) tea.Msg { return gkeLogsRefreshTickMsg{} })
}

// Update / View / SetSize / HasTextInputFocused are filled in by Task 11.
func (s *gkeLogs) Update(msg tea.Msg) tea.Cmd { _ = msg; return nil } // TODO: Task 11
func (s *gkeLogs) View() string                { return "" }          // TODO: Task 11
func (s *gkeLogs) SetSize(w, h int)            { s.width = w; _ = h }
func (s *gkeLogs) HasTextInputFocused() bool   { return false }
