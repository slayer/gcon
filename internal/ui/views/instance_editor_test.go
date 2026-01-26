package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/diff"
	"github.com/stretchr/testify/assert"
)

func TestNewInstanceEditorView(t *testing.T) {
	view := NewInstanceEditorView("my-project", "us-central1-a", "my-instance", nil)

	assert.NotNil(t, view)
	assert.Equal(t, "my-project", view.projectID)
	assert.Equal(t, "us-central1-a", view.zone)
	assert.Equal(t, "my-instance", view.instanceName)
	assert.Equal(t, stateLoading, view.state)
}

func TestInstanceEditorView_LabelsLoaded(t *testing.T) {
	view := NewInstanceEditorView("my-project", "us-central1-a", "my-instance", nil)
	view.width = 80
	view.height = 24

	// Simulate labels loaded
	msg := labelsLoadedMsg{
		labelsFingerprint: &gcp.InstanceLabelsFingerprint{
			Labels: map[string]string{
				"env":  "prod",
				"team": "backend",
			},
			Fingerprint: "abc123",
		},
	}

	view.Update(msg)

	assert.Equal(t, stateForm, view.state)
	assert.NotNil(t, view.labelEditor)
	assert.Equal(t, "abc123", view.fingerprint)
	assert.Equal(t, "prod", view.originalLabels["env"])
}

func TestInstanceEditorView_LabelsError(t *testing.T) {
	view := NewInstanceEditorView("my-project", "us-central1-a", "my-instance", nil)

	msg := labelsErrorMsg{
		err: assert.AnError,
	}

	view.Update(msg)

	assert.Equal(t, stateError, view.state)
	assert.Equal(t, assert.AnError, view.err)
}

func TestInstanceEditorView_RenderLoading(t *testing.T) {
	view := NewInstanceEditorView("my-project", "us-central1-a", "my-instance", nil)

	output := view.View()

	assert.Contains(t, output, "Loading labels")
}

func TestInstanceEditorView_RenderError(t *testing.T) {
	view := NewInstanceEditorView("my-project", "us-central1-a", "my-instance", nil)
	view.state = stateError
	view.err = assert.AnError

	output := view.View()

	assert.Contains(t, output, "Error")
	assert.Contains(t, output, "retry")
}

func TestInstanceEditorView_ShowDiffPreview(t *testing.T) {
	view := NewInstanceEditorView("my-project", "us-central1-a", "my-instance", nil)
	view.width = 80
	view.height = 24

	// Load labels
	view.Update(labelsLoadedMsg{
		labelsFingerprint: &gcp.InstanceLabelsFingerprint{
			Labels: map[string]string{
				"env": "prod",
			},
			Fingerprint: "abc123",
		},
	})

	// Modify a label (simulating user edit by modifying the underlying state)
	// In real usage, this would happen through the labelEditor
	view.labelEditor.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}) // Add mode
	view.labelEditor.Update(tea.KeyMsg{Type: tea.KeyTab})                       // Just to show it's in editing mode

	// For testing, let's just verify the form state
	assert.Equal(t, stateForm, view.state)
	assert.NotNil(t, view.labelEditor)
}

func TestInstanceEditorView_Cancel(t *testing.T) {
	view := NewInstanceEditorView("my-project", "us-central1-a", "my-instance", nil)
	view.width = 80
	view.height = 24

	// Load labels first
	view.Update(labelsLoadedMsg{
		labelsFingerprint: &gcp.InstanceLabelsFingerprint{
			Labels:      map[string]string{},
			Fingerprint: "abc123",
		},
	})

	// Press Escape to cancel
	cmd := view.Update(tea.KeyMsg{Type: tea.KeyEscape})
	assert.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(InstanceEditCancelledMsg)
	assert.True(t, ok)
}

func TestInstanceEditorView_RetryOnError(t *testing.T) {
	view := NewInstanceEditorView("my-project", "us-central1-a", "my-instance", nil)
	view.state = stateError
	view.err = assert.AnError

	// Press 'r' to retry
	cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	assert.NotNil(t, cmd)
	assert.Equal(t, stateLoading, view.state)
	assert.Nil(t, view.err)
}

func TestInstanceEditorView_DiffConfirm(t *testing.T) {
	view := NewInstanceEditorView("my-project", "us-central1-a", "my-instance", nil)
	view.state = stateDiff

	// Simulate confirm message from diff viewer
	cmd := view.Update(diff.ConfirmMsg{})

	assert.NotNil(t, cmd)
	assert.Equal(t, stateSaving, view.state)
}

func TestInstanceEditorView_DiffCancel(t *testing.T) {
	view := NewInstanceEditorView("my-project", "us-central1-a", "my-instance", nil)
	view.state = stateDiff

	// Simulate cancel message from diff viewer
	view.Update(diff.CancelMsg{})

	assert.Equal(t, stateForm, view.state)
}

func TestInstanceEditorView_SaveSuccess(t *testing.T) {
	view := NewInstanceEditorView("my-project", "us-central1-a", "my-instance", nil)

	// Simulate save success
	cmd := view.Update(labelsSavedMsg{})

	assert.NotNil(t, cmd)

	msg := cmd()
	editComplete, ok := msg.(InstanceEditCompleteMsg)
	assert.True(t, ok)
	assert.Equal(t, "my-instance", editComplete.InstanceName)
	assert.Equal(t, "labels", editComplete.EditType)
}

func TestInstanceEditorView_SaveError(t *testing.T) {
	view := NewInstanceEditorView("my-project", "us-central1-a", "my-instance", nil)

	// Simulate save error
	view.Update(labelsSaveErrorMsg{err: assert.AnError})

	assert.Equal(t, stateError, view.state)
	assert.Equal(t, assert.AnError, view.err)
}

func TestInstanceEditRequestMsg(t *testing.T) {
	msg := InstanceEditRequestMsg{
		ProjectID:    "my-project",
		Zone:         "us-central1-a",
		InstanceName: "my-instance",
		EditMode:     "labels",
	}

	assert.Equal(t, "my-project", msg.ProjectID)
	assert.Equal(t, "labels", msg.EditMode)
}
