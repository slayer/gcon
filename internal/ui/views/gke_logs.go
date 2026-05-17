package views

import (
	gocontext "context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
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
	// currentQuery is the LQL captured by the first fetch of the current
	// page chain — LoadMore reuses it so the `timestamp >= now-1h` lower
	// bound doesn't drift between pages. Cleared on Refresh so the next
	// first fetch re-anchors to the new "now".
	currentQuery string
	err          error
	loading      bool
	// generation increments on every Refresh (toggle / resource change /
	// manual refresh). Each fetch tags its response with the current
	// value so stale first-page or load-more responses can't appear
	// under a freshly-changed toolbar state.
	generation int

	spinner spinner.Model

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
		// One persistent spinner that survives across View() calls and
		// gets advanced by spinner.TickMsg in Update — the previous code
		// constructed a fresh spinner per render which never animated.
		spinner: components.NewGCPSpinner(),
	}
}

// SetTabActive marks the sub-view active/inactive so pending auto-refresh
// ticks can be ignored after the user leaves the tab.
func (s *gkeLogs) SetTabActive(active bool) { s.tabActive = active }

func (s *gkeLogs) Init() tea.Cmd {
	s.loading = true
	s.generation++
	return tea.Batch(s.spinner.Tick, s.fetchLogs(), s.tickAutoRefresh())
}

// Refresh re-runs the current filter without touching toggle state. Resets
// the pagination token AND the entries slice — without the latter, the
// view continues rendering log lines from the previous filter while the
// new request is in flight, making it look like stale results match the
// new toggles. Also clears the captured currentQuery so the next first
// fetch re-anchors the timestamp lower bound to the current "now".
func (s *gkeLogs) Refresh() tea.Cmd {
	s.loading = true
	s.err = nil
	s.entries = nil
	s.nextPageToken = ""
	s.loadingMore = false
	s.currentQuery = ""
	s.generation++
	return tea.Batch(s.spinner.Tick, s.fetchLogs())
}

func (s *gkeLogs) fetchLogs() tea.Cmd {
	if s.gcpClient == nil {
		return nil
	}
	// Anchor the query string for this page chain. LoadMore reuses
	// currentQuery instead of rebuilding via buildLQL — otherwise each
	// follow-up page would see a fresh "now-1h" lower bound, drifting
	// past entries the first page covered.
	s.currentQuery = s.buildLQL()
	query := s.currentQuery
	projectID := s.projectID
	gen := s.generation
	return func() tea.Msg {
		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 30*time.Second)
		defer cancel()
		lc, err := s.gcpClient.GetLoggingClient(projectID)
		if err != nil {
			return gkeLogsErrorMsg{gen: gen, err: fmt.Errorf("logging client: %w", err)}
		}
		entries, nextToken, err := lc.ListLogEntries(ctx, query, 100, "")
		if err != nil {
			return gkeLogsErrorMsg{gen: gen, err: err}
		}
		return gkeLogsLoadedMsg{gen: gen, entries: entries, nextPageToken: nextToken}
	}
}

// LoadMore fetches the next page of log entries using the pagination
// token from the previous fetch and APPENDS them to s.entries. Called by
// the parent details view when the user scrolls the viewport to the
// bottom. Returns nil when no more pages exist or a fetch is already in
// flight.
//
// Reuses the captured currentQuery so the timestamp lower bound doesn't
// drift between pages — buildLQL bakes in time.Now(), and rebuilding on
// each LoadMore would silently shrink the result window as time
// advances.
func (s *gkeLogs) LoadMore() tea.Cmd {
	if s.loadingMore || s.nextPageToken == "" || s.gcpClient == nil {
		return nil
	}
	s.loadingMore = true
	query := s.currentQuery
	if query == "" {
		// Defensive fallback: should never happen because LoadMore
		// requires a non-empty nextPageToken which is only set by a
		// preceding fetchLogs call that captured currentQuery.
		query = s.buildLQL()
	}
	token := s.nextPageToken
	projectID := s.projectID
	gen := s.generation
	return func() tea.Msg {
		ctx, cancel := gocontext.WithTimeout(gocontext.Background(), 30*time.Second)
		defer cancel()
		lc, err := s.gcpClient.GetLoggingClient(projectID)
		if err != nil {
			return gkeLogsErrorMsg{gen: gen, err: fmt.Errorf("logging client: %w", err)}
		}
		entries, nextToken, err := lc.ListLogEntries(ctx, query, 100, token)
		if err != nil {
			return gkeLogsErrorMsg{gen: gen, err: err}
		}
		return gkeLogsMoreLoadedMsg{gen: gen, entries: entries, nextPageToken: nextToken}
	}
}

// buildLQL composes the Cloud Logging filter from the sub-view's toggle state.
// Severity toggles are INDEPENDENT — each selected severity contributes a
// disjunct to a single bucket. Disabling a level genuinely excludes it
// server-side; enabling only ERROR no longer leaks WARNING/INFO into the
// response. When every toggle is off we emit an impossible filter so the
// API returns no rows (matches the View()'s "no toggles selected" state).
//
// A 1h timestamp lower-bound is appended so each refresh / auto-refresh
// tick stays cheap. Mirrors the standalone Logs Explorer's
// buildEffectiveQuery default; if pagination ever runs to >1h ago, the
// LoadMore() path inherits the same bound.
func (s *gkeLogs) buildLQL() string {
	parts := []string{
		fmt.Sprintf(`resource.type = %q`, s.resourceType),
		fmt.Sprintf(`resource.labels.cluster_name = %q`, s.clusterName),
		fmt.Sprintf(`resource.labels.location = %q`, s.location),
		fmt.Sprintf(`timestamp >= %q`, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)),
	}
	var sevPredicates []string
	if s.infoOn {
		// INFO bucket also captures NOTICE / DEBUG / DEFAULT — anything
		// non-warning, non-error. `severity < WARNING` is the cleanest
		// expression and matches what Cloud Logging's own UI dropdown does.
		sevPredicates = append(sevPredicates, `severity < WARNING`)
	}
	if s.warnOn {
		sevPredicates = append(sevPredicates, `severity = WARNING`)
	}
	if s.errOn {
		sevPredicates = append(sevPredicates, `severity >= ERROR`)
	}
	if len(sevPredicates) == 0 {
		// All toggles off — match nothing.
		parts = append(parts, `severity = "NEVERMATCH"`)
	} else {
		parts = append(parts, "("+strings.Join(sevPredicates, " OR ")+")")
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
	case spinner.TickMsg:
		// Only advance / re-schedule while loading — otherwise the tick
		// loop runs forever and drives needless redraws.
		if !s.loading && !s.loadingMore {
			return nil
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(m)
		return cmd
	case gkeLogsLoadedMsg:
		// Drop responses from a superseded query — e.g. a slow first-page
		// fetch that lands after the user toggled severity off and a new
		// fetch is already in flight. Otherwise stale entries appear
		// under the new toolbar state.
		if m.gen != s.generation {
			return nil
		}
		s.loading = false
		s.entries = m.entries
		s.nextPageToken = m.nextPageToken
		return nil
	case gkeLogsMoreLoadedMsg:
		// Same guard for infinite-scroll pages: a LoadMore that arrives
		// after a Refresh would otherwise tack stale entries onto the
		// new (empty) list.
		if m.gen != s.generation {
			return nil
		}
		s.loadingMore = false
		s.entries = append(s.entries, m.entries...)
		s.nextPageToken = m.nextPageToken
		return nil
	case gkeLogsErrorMsg:
		if m.gen != s.generation {
			return nil
		}
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
	case "R":
		// Cycle through the Kubernetes resource types. A full popover-style
		// dropdown isn't worth the wiring for four options — cycling lets
		// the user mash R until they hit the scope they want.
		s.resourceType = nextGKELogsResourceType(s.resourceType)
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

// gkeLogsResourceTypes is the ordered list cycled through by the `R` key.
// k8s_cluster is the broadest scope (cluster-level events like autoscaler
// decisions); each subsequent type narrows down to a finer-grained
// resource. After k8s_container the cycle wraps back to k8s_cluster.
var gkeLogsResourceTypes = []string{
	"k8s_cluster",
	"k8s_node",
	"k8s_pod",
	"k8s_container",
}

// nextGKELogsResourceType returns the next resource type in the cycle,
// wrapping after the last entry. Returns the first entry if the current
// value isn't in the list (defensive — happens only if a future caller
// stores an unrecognized type).
func nextGKELogsResourceType(current string) string {
	for i, rt := range gkeLogsResourceTypes {
		if rt == current {
			return gkeLogsResourceTypes[(i+1)%len(gkeLogsResourceTypes)]
		}
	}
	return gkeLogsResourceTypes[0]
}

// enabledSeverities lists severity tokens for any toggle currently on.
// The dedicated Logs Explorer accepts these to pre-check its severity
// filter dropdown, then adds its own `severity = ...` filter on top of
// the LQL we hand over. We must list EVERY level each toggle covers —
// otherwise the Explorer's filter narrows the result beyond what the
// embedded view shows. The INFO toggle covers INFO/NOTICE/DEBUG/DEFAULT
// (anything below WARNING); the ERROR toggle covers
// ERROR/CRITICAL/ALERT/EMERGENCY (anything ≥ ERROR). Keep this in lockstep
// with the disjuncts in buildLQL.
func (s *gkeLogs) enabledSeverities() []string {
	var out []string
	if s.infoOn {
		out = append(out, "DEFAULT", "DEBUG", "INFO", "NOTICE")
	}
	if s.warnOn {
		out = append(out, "WARNING")
	}
	if s.errOn {
		out = append(out, "ERROR", "CRITICAL", "ALERT", "EMERGENCY")
	}
	return out
}

func (s *gkeLogs) View() string {
	// The toolbar renders FIRST in every state so the user can see the
	// effect of severity / resource-type / auto-refresh toggles even
	// while a refresh is in flight. The previous layout swapped the
	// whole tab for "Loading logs..." after a toggle press, hiding the
	// updated toggle indicator and leaving the user unsure their `I` /
	// `W` / `E` keystroke registered.
	var b strings.Builder
	b.WriteString(s.renderToolbar())
	b.WriteString("\n\n")

	if s.loading && len(s.entries) == 0 {
		b.WriteString(renderLoading(s.spinner, "Loading logs..."))
		return b.String()
	}
	if s.err != nil && len(s.entries) == 0 {
		b.WriteString(components.RenderError(s.err))
		return b.String()
	}
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
	// A LoadMore error takes precedence over the scroll hint so pagination
	// failures aren't silent (the renderer used to only show errors when
	// the entries slice was empty, hiding LoadMore failures entirely).
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	switch {
	case s.err != nil:
		b.WriteString(errStyle.Render(fmt.Sprintf("  ⚠ load more failed: %v — press 'r' to retry", s.err)))
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
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368")).Faint(true)
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
	state := fmt.Sprintf("Severity: %s %s %s    Resource: %s    Auto-refresh: %s",
		toggle("INFO", s.infoOn),
		toggle("WARNING", s.warnOn),
		toggle("ERROR", s.errOn),
		s.resourceType,
		autoState,
	)
	// Second line: keybinding hints. Dimmed so they sit below the live
	// state without competing for attention. New users discover the
	// toggles without having to leave the tab to check key-bindings.md.
	help := dim.Render(
		"  I/W/E toggle severity   R cycle resource   a auto-refresh   r refresh   L open Logs Explorer",
	)
	return state + "\n" + help
}

// formatGKELogEntry renders one entry as a single line: timestamp + severity
// + raw message. The parent details view's viewport handles overflow via
// horizontal scroll; we deliberately don't truncate at the data layer so
// users can see the full payload by scrolling. Multi-line / structured
// payload rendering can be added in a later phase.
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
