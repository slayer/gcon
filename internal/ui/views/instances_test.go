package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
)

func TestInstancesView_NewInstancesView(t *testing.T) {
	v := NewInstancesView("test-project")

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.True(t, v.loading, "View should start in loading state")
}

func TestInstancesView_RenderLoading(t *testing.T) {
	v := NewInstancesView("test-project")
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30}
	v.SetContext(ctx)

	// renderLoading should return a simple loading message
	// Height enforcement is now handled at the app level
	output := renderLoading(v.spinner, "Loading...")

	assert.Contains(t, output, "Loading...")
	assert.Contains(t, output, v.spinner.View())
}

func TestInstances_TKey_OpensSSHDialog_OnRunningRow(t *testing.T) {
	v := NewInstancesView("proj")
	v.Update(instancesLoadedMsg{instances: []gcp.Instance{
		{Name: "vm", Status: "RUNNING", InternalIP: "10.0.0.5", ExternalIP: "34.1.2.3",
			Zone: "us-central1-a"},
	}})
	v.table.SetCursor(0)
	_ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.True(t, v.showSSHDialog)
	assert.NotNil(t, v.sshDialog)
}

func TestInstances_TKey_NoOp_OnStoppedRow(t *testing.T) {
	v := NewInstancesView("proj")
	v.Update(instancesLoadedMsg{instances: []gcp.Instance{
		{Name: "vm", Status: "TERMINATED"},
	}})
	v.table.SetCursor(0)
	_ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.False(t, v.showSSHDialog)
}

func TestInstances_IsMenuOpen_TrueWhenSSHDialogOpen(t *testing.T) {
	v := NewInstancesView("proj")
	v.Update(instancesLoadedMsg{instances: []gcp.Instance{
		{Name: "vm", Status: "RUNNING", InternalIP: "10.0.0.5", Zone: "us-central1-a"},
	}})
	v.table.SetCursor(0)
	assert.False(t, v.IsMenuOpen())
	_ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.True(t, v.IsMenuOpen())
}
