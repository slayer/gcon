package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/gcp"
)

func TestGKELogs_DefaultLQL(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	q := s.buildLQL()
	assert.Contains(t, q, `resource.type = "k8s_cluster"`)
	assert.Contains(t, q, `resource.labels.cluster_name = "prod"`)
	assert.Contains(t, q, `resource.labels.location = "us-central1"`)
	// Default state has all severity toggles on, producing three OR'd
	// predicates rather than a single `severity >= INFO`.
	assert.Contains(t, q, `severity < WARNING`)
	assert.Contains(t, q, `severity = WARNING`)
	assert.Contains(t, q, `severity >= ERROR`)
}

func TestGKELogs_SeverityToggle(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.infoOn = false
	s.warnOn = false
	q := s.buildLQL()
	assert.Contains(t, q, `severity >= ERROR`)
}

func TestGKELogs_ResourceTypeSwitch(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.resourceType = "k8s_pod"
	q := s.buildLQL()
	assert.Contains(t, q, `resource.type = "k8s_pod"`)
}

func TestGKELogs_AutoRefreshOffByDefault(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	assert.False(t, s.autoRefresh)
}

func TestGKELogs_StaleTickDropped(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.tabActive = false
	s.autoRefresh = true
	cmd := s.Update(gkeLogsRefreshTickMsg{})
	assert.Nil(t, cmd)
}

func TestGKELogs_KeyToggleInfo(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	assert.True(t, s.infoOn)
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}})
	assert.False(t, s.infoOn)
}

// TestGKELogs_FirstFetchStoresPageToken verifies that gkeLogsLoadedMsg
// seeds nextPageToken — required for LoadMore to fire on subsequent scroll.
func TestGKELogs_FirstFetchStoresPageToken(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.Update(gkeLogsLoadedMsg{
		gen:           0,
		entries:       []gcp.LogEntry{{Message: "first"}},
		nextPageToken: "page-2",
	})
	assert.Equal(t, "page-2", s.nextPageToken,
		"first fetch must persist the pagination token for LoadMore")
	assert.False(t, s.loading)
}

// TestGKELogs_MoreLoadedAppendsEntries verifies that follow-up pages
// append (not replace) and refresh the token.
func TestGKELogs_MoreLoadedAppendsEntries(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.entries = []gcp.LogEntry{{Message: "first"}}
	s.nextPageToken = "page-2"
	s.loadingMore = true
	s.Update(gkeLogsMoreLoadedMsg{
		gen:           0,
		entries:       []gcp.LogEntry{{Message: "second"}},
		nextPageToken: "page-3",
	})
	assert.Len(t, s.entries, 2, "second page must append, not replace")
	assert.Equal(t, "second", s.entries[1].Message)
	assert.Equal(t, "page-3", s.nextPageToken)
	assert.False(t, s.loadingMore, "loadingMore flag must clear so next scroll re-triggers")
}

// TestGKELogs_StaleResponseDropped guards against the regression where a
// slow first-page or LoadMore response could appear under a freshly
// changed toolbar state (severity / resource toggle has bumped the
// generation).
func TestGKELogs_StaleResponseDropped(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	// User toggled severity — generation is now 1, entries cleared, loading=true.
	s.generation = 1
	s.loading = true
	s.entries = nil

	// Slow response from the previous (gen=0) fetch lands.
	s.Update(gkeLogsLoadedMsg{
		gen:     0,
		entries: []gcp.LogEntry{{Message: "stale"}},
	})
	assert.Empty(t, s.entries, "stale-gen response must not populate entries")
	assert.True(t, s.loading, "stale-gen response must not flip loading=false")

	// LoadMore from the same stale generation also dropped.
	s.entries = []gcp.LogEntry{{Message: "fresh"}}
	s.Update(gkeLogsMoreLoadedMsg{
		gen:     0,
		entries: []gcp.LogEntry{{Message: "stale-more"}},
	})
	assert.Len(t, s.entries, 1, "stale-gen LoadMore must not append")
}

// TestGKELogs_EnabledSeveritiesCoversFullBuckets guards against the
// regression where enabledSeverities returned only INFO/WARNING/ERROR
// — the Logs Explorer would then exclude DEBUG/NOTICE/CRITICAL/ALERT/
// EMERGENCY events that ARE shown in the embedded view.
func TestGKELogs_EnabledSeveritiesCoversFullBuckets(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	sevs := s.enabledSeverities()
	for _, want := range []string{"DEBUG", "INFO", "NOTICE", "DEFAULT", "WARNING", "ERROR", "CRITICAL", "ALERT", "EMERGENCY"} {
		assert.Contains(t, sevs, want, "default state (all on) must include %s", want)
	}

	s.infoOn = false
	s.warnOn = false
	sevs = s.enabledSeverities()
	assert.NotContains(t, sevs, "INFO")
	assert.NotContains(t, sevs, "DEBUG")
	assert.NotContains(t, sevs, "WARNING")
	assert.Contains(t, sevs, "ERROR")
	assert.Contains(t, sevs, "CRITICAL")
	assert.Contains(t, sevs, "EMERGENCY")
}

// TestGKELogs_LoadMoreNoOpWithoutToken guards against firing duplicate or
// invalid pagination requests.
func TestGKELogs_LoadMoreNoOpWithoutToken(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.nextPageToken = ""
	assert.Nil(t, s.LoadMore(), "no token → no fetch")

	s.nextPageToken = "page-2"
	s.loadingMore = true
	assert.Nil(t, s.LoadMore(), "already loading → no duplicate fetch")
}

// TestGKELogs_SeverityIndependent confirms that severity toggles produce
// per-level predicates joined by OR rather than the previous "lowest
// threshold wins" shortcut (which leaked higher-severity rows when the
// user wanted INFO-only and dropped INFO when only WARNING was selected).
func TestGKELogs_SeverityIndependent(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	// INFO only — must EXCLUDE WARNING and ERROR.
	s.infoOn = true
	s.warnOn = false
	s.errOn = false
	q := s.buildLQL()
	assert.Contains(t, q, "severity < WARNING")
	assert.NotContains(t, q, "severity = WARNING")
	assert.NotContains(t, q, "severity >= ERROR")

	// ERROR only — must NOT match INFO/WARNING.
	s.infoOn = false
	s.errOn = true
	q = s.buildLQL()
	assert.Contains(t, q, "severity >= ERROR")
	assert.NotContains(t, q, "severity < WARNING")

	// WARNING + ERROR — both predicates present, joined by OR.
	s.warnOn = true
	q = s.buildLQL()
	assert.Contains(t, q, "severity = WARNING")
	assert.Contains(t, q, "severity >= ERROR")
	assert.Contains(t, q, " OR ")

	// All toggles off — must emit an impossible filter so nothing matches.
	s.infoOn = false
	s.warnOn = false
	s.errOn = false
	q = s.buildLQL()
	assert.Contains(t, q, "NEVERMATCH")
}

// TestGKELogs_ToolbarVisibleWhileLoading guards against the regression
// where Refresh blanked the toolbar with "Loading logs...", hiding the
// severity / resource / auto-refresh toggle state. The user pressed I/W/E
// and saw no feedback that the toggle had flipped.
func TestGKELogs_ToolbarVisibleWhileLoading(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	s.loading = true
	s.entries = nil
	out := s.View()
	assert.Contains(t, out, "Loading logs",
		"loading message must still render in the body")
	assert.Contains(t, out, "Severity:",
		"toolbar must remain visible during loading so toggle feedback is reachable")
	assert.Contains(t, out, "INFO")
	assert.Contains(t, out, "WARNING")
	assert.Contains(t, out, "ERROR")
}

// TestGKELogs_RCyclesResourceType verifies the R-key handler advances
// through the resource-type cycle and wraps after the last entry.
func TestGKELogs_RCyclesResourceType(t *testing.T) {
	s := newGKELogs("proj", "us-central1", "prod", nil)
	assert.Equal(t, "k8s_cluster", s.resourceType)
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	assert.Equal(t, "k8s_node", s.resourceType)
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	assert.Equal(t, "k8s_pod", s.resourceType)
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	assert.Equal(t, "k8s_container", s.resourceType)
	// Wraps back to the first entry.
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	assert.Equal(t, "k8s_cluster", s.resourceType)
}
