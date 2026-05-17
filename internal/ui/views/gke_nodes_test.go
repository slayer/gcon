package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

func gkeNodesWithFixtures() *gkeNodes {
	s := newGKENodes("proj", &gcp.ClusterDetails{
		Cluster: gcp.Cluster{Name: "prod", Location: "us-central1"},
		NodePools: []gcp.NodePool{
			{Name: "default"},
			{Name: "gpu-pool"},
		},
	}, nil)
	s.loading = false
	s.SetTabActive(true)
	s.nodes = []gcp.GKENode{
		{MIGInstance: gcp.MIGInstance{Name: "gke-prod-default-7f3a-abcd", Zone: "us-central1-a", Status: "RUNNING", InternalIP: "10.128.0.12", CreatedAt: "2026-05-01T12:00:00Z"}, Pool: "default"},
		{MIGInstance: gcp.MIGInstance{Name: "gke-prod-default-7f3a-efgh", Zone: "us-central1-b", Status: "RUNNING", InternalIP: "10.128.0.13", CreatedAt: "2026-05-01T12:00:00Z"}, Pool: "default"},
		{MIGInstance: gcp.MIGInstance{Name: "gke-prod-gpu-pool-2c1f-xyz1", Zone: "us-central1-a", Status: "STAGING", InternalIP: "10.128.0.27", CreatedAt: "2026-05-15T08:00:00Z"}, Pool: "gpu-pool"},
	}
	s.SetSize(160, 40)
	s.refreshTable()
	return s
}

func TestGKENodes_RendersRows(t *testing.T) {
	s := gkeNodesWithFixtures()
	out := s.View()
	assert.Contains(t, out, "gke-prod-default-7f3a-abcd")
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "gpu-pool")
	assert.Contains(t, out, "RUNNING")
	assert.Contains(t, out, "STAGING")
	assert.Contains(t, out, "10.128.0.12")
}

func TestGKENodes_FilterByPool(t *testing.T) {
	s := gkeNodesWithFixtures()
	s.table.SetFilter("pool:gpu-pool")
	out := s.View()
	assert.Contains(t, out, "gke-prod-gpu-pool-2c1f-xyz1")
	assert.NotContains(t, out, "gke-prod-default-7f3a-abcd")
}

// TestGKENodes_EnterEmitsInstanceSelected verifies that pressing Enter
// on a focused row emits InstanceSelectedMsg with the node's Name and Zone
// populated. The app-level handler (handleInstanceSelected at
// app_navigation.go:446) only reads Instance.Name and Instance.Zone, so a
// minimal gcp.Instance is sufficient.
func TestGKENodes_EnterEmitsInstanceSelected(t *testing.T) {
	s := gkeNodesWithFixtures()
	// Cursor starts at the first row after refreshTable (sorted by name).
	// After dedupeAndSort, the order is alphabetical by Name:
	//   gke-prod-default-7f3a-abcd
	//   gke-prod-default-7f3a-efgh
	//   gke-prod-gpu-pool-2c1f-xyz1
	cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "Enter should emit a command")
	msg := cmd()
	selected, ok := msg.(InstanceSelectedMsg)
	require.True(t, ok, "expected InstanceSelectedMsg, got %T", msg)
	assert.Equal(t, "gke-prod-default-7f3a-abcd", selected.Instance.Name)
	assert.Equal(t, "us-central1-a", selected.Instance.Zone)
}

// TestGKENodes_CursorMovementPersists guards against a regression where
// handleKey discarded the new table.Model returned by table.Update (value
// receiver), so cursor movement was applied and immediately thrown away.
// Symptom in the live UI: j/k didn't move the cursor and Enter always
// selected the first row.
func TestGKENodes_CursorMovementPersists(t *testing.T) {
	s := gkeNodesWithFixtures()
	// Sorted order: abcd, efgh, gpu-pool-...xyz1. Cursor starts at row 0.
	s.Update(tea.KeyMsg{Type: tea.KeyDown})
	cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	selected, ok := msg.(InstanceSelectedMsg)
	require.True(t, ok)
	assert.Equal(t, "gke-prod-default-7f3a-efgh", selected.Instance.Name,
		"Down then Enter must select the second row, not the first")
}

// TestGKENodes_SortMenuOpensOnS guards against the regression where every
// column was created with Sortable:false, making the documented S-sort
// menu a silent no-op (the shared table widget only opens the menu when
// at least one column has Sortable set).
func TestGKENodes_SortMenuOpensOnS(t *testing.T) {
	s := gkeNodesWithFixtures()
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	assert.True(t, s.table.IsSortMenuOpen(),
		"S must open the sort menu — requires at least one column with Sortable:true")
}

// TestGKENodes_EnterWhileFilteringGoesToTable verifies that Enter while
// the filter input is focused reaches the table widget (which closes /
// applies the filter), instead of being intercepted to navigate to an
// instance.
func TestGKENodes_EnterWhileFilteringGoesToTable(t *testing.T) {
	s := gkeNodesWithFixtures()
	// Open the filter input — '/' is the table widget's filter shortcut.
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	require.True(t, s.table.HasTextInputFocused(), "filter input should be focused after '/'")

	cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// If Enter were misrouted to "open instance details", cmd would
	// return InstanceSelectedMsg. The table consumes Enter to apply the
	// filter and returns nil. Either way, the cursor must not have
	// emitted a navigation message.
	if cmd != nil {
		msg := cmd()
		_, isNav := msg.(InstanceSelectedMsg)
		assert.False(t, isNav, "Enter while filter input is focused must not emit InstanceSelectedMsg")
	}
}

// TestGKENodes_RefreshClearsStaleRows guards against the regression where
// Init() reset s.nodes but left the table rendering the previous run's
// rows; on total fan-out failure the user saw old rows under an error
// banner that looked authoritative.
func TestGKENodes_RefreshClearsStaleRows(t *testing.T) {
	s := gkeNodesWithFixtures()
	// Sanity: fixture populated rows.
	require.NotEmpty(t, s.table.Rows())
	// Refresh — no compute client wired, so init returns early at fanOut.
	s.Refresh()
	assert.Empty(t, s.table.Rows(), "Refresh must clear stale rows before the new fetch lands")
}
