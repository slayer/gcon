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
	assert.Contains(t, q, `severity >= INFO`)
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
		entries:       []gcp.LogEntry{{Message: "second"}},
		nextPageToken: "page-3",
	})
	assert.Len(t, s.entries, 2, "second page must append, not replace")
	assert.Equal(t, "second", s.entries[1].Message)
	assert.Equal(t, "page-3", s.nextPageToken)
	assert.False(t, s.loadingMore, "loadingMore flag must clear so next scroll re-triggers")
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
