package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLInstancesView_StopKey(t *testing.T) {
	v := NewSQLInstancesView("test-project")

	// Simulate client ready and instances loaded
	v.Update(sqlClientReadyMsg{client: &gcp.SQLClient{}})
	v.Update(sqlInstancesLoadedMsg{instances: []gcp.SQLInstance{
		{Name: "my-db", State: "RUNNABLE", DatabaseVersion: "POSTGRES_15", Region: "us-central1", Tier: "db-f1-micro"},
	}})

	// Press 'x' to stop
	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	// Should return a command that produces SQLInstanceActionMsg
	require.NotNil(t, cmd, "pressing 'x' on a RUNNABLE instance should return a cmd")

	msg := cmd()
	actionMsg, ok := msg.(SQLInstanceActionMsg)
	require.True(t, ok, "cmd should produce SQLInstanceActionMsg, got %T", msg)
	assert.Equal(t, "stop", actionMsg.Action)
	assert.Equal(t, "my-db", actionMsg.InstanceName)
}

func TestSQLInstancesView_StartKey(t *testing.T) {
	v := NewSQLInstancesView("test-project")

	v.Update(sqlClientReadyMsg{client: &gcp.SQLClient{}})
	v.Update(sqlInstancesLoadedMsg{instances: []gcp.SQLInstance{
		{Name: "my-db", State: "STOPPED", DatabaseVersion: "POSTGRES_15", Region: "us-central1", Tier: "db-f1-micro"},
	}})

	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	require.NotNil(t, cmd, "pressing 's' on a STOPPED instance should return a cmd")

	msg := cmd()
	actionMsg, ok := msg.(SQLInstanceActionMsg)
	require.True(t, ok, "cmd should produce SQLInstanceActionMsg, got %T", msg)
	assert.Equal(t, "start", actionMsg.Action)
	assert.Equal(t, "my-db", actionMsg.InstanceName)
}

func TestSQLInstancesView_RestartKey(t *testing.T) {
	v := NewSQLInstancesView("test-project")

	v.Update(sqlClientReadyMsg{client: &gcp.SQLClient{}})
	v.Update(sqlInstancesLoadedMsg{instances: []gcp.SQLInstance{
		{Name: "my-db", State: "RUNNABLE", DatabaseVersion: "POSTGRES_15", Region: "us-central1", Tier: "db-f1-micro"},
	}})

	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})

	require.NotNil(t, cmd, "pressing 'R' on a RUNNABLE instance should return a cmd")

	msg := cmd()
	actionMsg, ok := msg.(SQLInstanceActionMsg)
	require.True(t, ok, "cmd should produce SQLInstanceActionMsg, got %T", msg)
	assert.Equal(t, "restart", actionMsg.Action)
	assert.Equal(t, "my-db", actionMsg.InstanceName)
}

func TestSQLInstancesView_ActionMenu_Stop(t *testing.T) {
	v := NewSQLInstancesView("test-project")

	v.Update(sqlClientReadyMsg{client: &gcp.SQLClient{}})
	v.Update(sqlInstancesLoadedMsg{instances: []gcp.SQLInstance{
		{Name: "my-db", State: "RUNNABLE", DatabaseVersion: "POSTGRES_15", Region: "us-central1", Tier: "db-f1-micro"},
	}})

	// Simulate executeAction from action menu
	cmd := v.executeAction('x')

	require.NotNil(t, cmd, "executeAction('x') on RUNNABLE should return a cmd")

	msg := cmd()
	actionMsg, ok := msg.(SQLInstanceActionMsg)
	require.True(t, ok, "cmd should produce SQLInstanceActionMsg, got %T", msg)
	assert.Equal(t, "stop", actionMsg.Action)
}

func TestSQLInstanceDetailsView_StopKey(t *testing.T) {
	v := NewSQLInstanceDetailsView("test-project", "my-db", &gcp.SQLClient{})

	// Set up view size first so rendering doesn't panic
	v.width = 80
	v.height = 40
	v.applySize(80, 40)

	// Simulate details loaded
	v.Update(sqlInstanceDetailsLoadedMsg{details: &gcp.SQLInstanceDetails{
		Name:            "my-db",
		State:           "RUNNABLE",
		DatabaseVersion: "POSTGRES_15",
		Region:          "us-central1",
		Tier:            "db-f1-micro",
	}})

	// Press 'x' to stop
	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	require.NotNil(t, cmd, "pressing 'x' on details of RUNNABLE instance should return a cmd")

	msg := cmd()
	actionMsg, ok := msg.(SQLInstanceActionMsg)
	require.True(t, ok, "cmd should produce SQLInstanceActionMsg, got %T: %v", msg, msg)
	assert.Equal(t, "stop", actionMsg.Action)
	assert.Equal(t, "my-db", actionMsg.InstanceName)
}
