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
