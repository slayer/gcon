package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/gcp"
)

func gkeListWithFixtures() *GKEClustersView {
	v := NewGKEClustersView("proj", nil)
	v.SetSize(160, 40)
	v.clusters = []gcp.Cluster{
		{Name: "prod", Location: "us-central1", LocationType: "region", Mode: "STANDARD", MasterVersion: "1.30.5", NodeCount: 12, Status: "RUNNING", NodeVersion: "1.30.5", NodeVersionsUniform: true, CreatedAt: "2025-08-12T14:03:00Z"},
		{Name: "stage", Location: "us-central1-a", LocationType: "zone", Mode: "AUTOPILOT", MasterVersion: "1.30.5", NodeCount: 0, Status: "RUNNING", NodeVersion: "1.30.5", NodeVersionsUniform: true, CreatedAt: "2025-08-12T14:03:00Z"},
	}
	v.refreshTable()
	return v
}

func TestGKEClustersView_RendersRows(t *testing.T) {
	v := gkeListWithFixtures()
	out := v.View()
	assert.Contains(t, out, "prod")
	assert.Contains(t, out, "stage")
	assert.Contains(t, out, "Standard")
	assert.Contains(t, out, "Autopilot")
	// Autopilot shows "(managed)" rather than 0 in the Nodes column.
	assert.Contains(t, out, "(managed)")
	// Status column must render — regression: bubbles/table byte-truncates
	// ANSI-styled cells mid-escape, stripping the visible text.
	assert.Contains(t, out, "RUNNING")
}

func TestGKEClustersView_AutopilotFilter(t *testing.T) {
	v := gkeListWithFixtures()
	v.table.SetFilter("mode:autopilot")
	out := v.View()
	assert.Contains(t, out, "stage")
	assert.NotContains(t, out, "prod")
}
