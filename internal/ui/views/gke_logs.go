package views

import (
	gocontext "context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

	entries       []gcp.LogEntry
	nextPageToken string // seeded by the first fetch; empty when no more pages
	loadingMore   bool   // gate to dedupe infinite-scroll triggers
	err           error
	loading       bool

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

// Refresh re-runs the current filter without touching toggle state. Resets
// the pagination token so scrolling fetches a fresh page chain.
func (s *gkeLogs) Refresh() tea.Cmd {
	s.loading = true
	s.err = nil
	s.nextPageToken = ""
	s.loadingMore = false
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
		entries, nextToken, err := lc.ListLogEntries(ctx, query, 100, "")
		if err != nil {
			return gkeLogsErrorMsg{err: err}
		}
		return gkeLogsLoadedMsg{entries: entries, nextPageToken: nextToken}
	}
}

// LoadMore fetches the next page of log entries using the pagination
// token from the previous fetch and APPENDS them to s.entries. Called by
// the parent details view when the user scrolls the viewport to the
// bottom. Returns nil when no more pages exist or a fetch is already in
// flight.
func (s *gkeLogs) LoadMore() tea.Cmd {
	if s.loadingMore || s.nextPageToken == "" || s.gcpClient == nil {
		return nil
	}
	s.loadingMore = true
	query := s.buildLQL()
	token := s.nextPageToken
	projectID := s.projectID
	return func() tea.Msg {
		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 30*time.Second)
		defer cancel()
		lc, err := s.gcpClient.GetLoggingClient(projectID)
		if err != nil {
			return gkeLogsErrorMsg{err: fmt.Errorf("logging client: %w", err)}
		}
		entries, nextToken, err := lc.ListLogEntries(ctx, query, 100, token)
		if err != nil {
			return gkeLogsErrorMsg{err: err}
		}
		return gkeLogsMoreLoadedMsg{entries: entries, nextPageToken: nextToken}
	}
}

// buildLQL composes the Cloud Logging filter from the sub-view's toggle state.
// Severity uses the lowest enabled level so the LQL stays simple — client-side
// filtering (Task 11) hides individual rows when only a higher level is on.
func (s *gkeLogs) buildLQL() string {
	parts := []string{
		fmt.Sprintf(`resource.type = %q`, s.resourceType),
		fmt.Sprintf(`resource.labels.cluster_name = %q`, s.clusterName),
		fmt.Sprintf(`resource.labels.location = %q`, s.location),
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

func (s *gkeLogs) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case gkeLogsLoadedMsg:
		s.loading = false
		s.entries = m.entries
		s.nextPageToken = m.nextPageToken
		return nil
	case gkeLogsMoreLoadedMsg:
		s.loadingMore = false
		s.entries = append(s.entries, m.entries...)
		s.nextPageToken = m.nextPageToken
		return nil
	case gkeLogsErrorMsg:
		s.loading = false
		s.loadingMore = false
		s.err = m.err
		return nil
	case gkeLogsRefreshTickMsg:
		if !s.autoRefresh || !s.tabActive {
			return nil
		}
		return tea.Batch(s.fetchLogs(), s.tickAutoRefresh())
	case tea.KeyMsg:
		return s.handleKey(m)
	}
	return nil
}

func (s *gkeLogs) handleKey(m tea.KeyMsg) tea.Cmd {
	switch m.String() {
	case "I":
		s.infoOn = !s.infoOn
		return s.Refresh()
	case "W":
		s.warnOn = !s.warnOn
		return s.Refresh()
	case "E":
		s.errOn = !s.errOn
		return s.Refresh()
	case "a":
		s.autoRefresh = !s.autoRefresh
		if s.autoRefresh {
			return s.tickAutoRefresh()
		}
	case "r":
		return s.Refresh()
	case "L":
		// Hand the cluster-scoped filter off to the dedicated Logs
		// Explorer. LogsViewRequestMsg is already wired by the Cloud Run
		// observability tab — Phase 2a reuses the same plumbing rather
		// than introducing a redundant message type.
		severities := s.enabledSeverities()
		query := s.buildLQL()
		return func() tea.Msg {
			return LogsViewRequestMsg{Query: query, Severities: severities}
		}
	}
	return nil
}

// enabledSeverities lists severity tokens for any toggle currently on.
// The dedicated Logs Explorer accepts these to pre-check its severity
// filter dropdown.
func (s *gkeLogs) enabledSeverities() []string {
	var out []string
	if s.infoOn {
		out = append(out, "INFO")
	}
	if s.warnOn {
		out = append(out, "WARNING")
	}
	if s.errOn {
		out = append(out, "ERROR")
	}
	return out
}

func (s *gkeLogs) View() string {
	if s.loading && len(s.entries) == 0 {
		return renderLoading(components.NewGCPSpinner(), "Loading logs...")
	}
	if s.err != nil && len(s.entries) == 0 {
		return components.RenderError(s.err)
	}
	var b strings.Builder
	b.WriteString(s.renderToolbar())
	b.WriteString("\n\n")
	if len(s.entries) == 0 {
		muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		b.WriteString(muted.Render("  (no log entries match the current filters)"))
		return b.String()
	}
	for i := range s.entries {
		b.WriteString(formatGKELogEntry(&s.entries[i]))
		b.WriteString("\n")
	}
	// Footer hint so the user knows whether scrolling further fetches more.
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	switch {
	case s.loadingMore:
		b.WriteString(muted.Render("  Loading more entries..."))
	case s.nextPageToken != "":
		b.WriteString(muted.Render("  (scroll to load more)"))
	default:
		b.WriteString(muted.Render("  (end of results)"))
	}
	return b.String()
}

func (s *gkeLogs) renderToolbar() string {
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	active := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	toggle := func(label string, on bool) string {
		if on {
			return active.Render("[x " + label + "]")
		}
		return muted.Render("[  " + label + "]")
	}
	autoState := "off"
	if s.autoRefresh {
		autoState = "on"
	}
	return fmt.Sprintf("Severity: %s %s %s    Resource: %s    Auto-refresh: %s (a)",
		toggle("INFO", s.infoOn),
		toggle("WARNING", s.warnOn),
		toggle("ERROR", s.errOn),
		s.resourceType,
		autoState,
	)
}

// formatGKELogEntry renders one entry as a single line: timestamp + severity
// + message (truncated to fit). Multi-line / structured payload rendering
// can be added in a later phase.
func formatGKELogEntry(e *gcp.LogEntry) string {
	ts := e.Timestamp.UTC().Format("2006-01-02 15:04:05")
	msg := e.TextPayload
	if msg == "" {
		msg = e.Message
	}
	if msg == "" {
		// Best-effort fallback if the entry uses JSON payload.
		if v, ok := e.JSONPayload["message"]; ok {
			msg = fmt.Sprint(v)
		}
	}
	sev := e.Severity
	if sev == "" {
		sev = "DEFAULT"
	}
	color := severityColorForGKE(sev)
	return fmt.Sprintf("  %s %s  %s", ts, color.Render(fmt.Sprintf("%-8s", sev)), msg)
}

func severityColorForGKE(sev string) lipgloss.Style {
	switch sev {
	case "ERROR", "CRITICAL", "ALERT", "EMERGENCY":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	case "WARNING":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))
	case "INFO", "NOTICE":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
}

func (s *gkeLogs) SetSize(w, h int)          { s.width = w; _ = h }
func (s *gkeLogs) HasTextInputFocused() bool { return false }
