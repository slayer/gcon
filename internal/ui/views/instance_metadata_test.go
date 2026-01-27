package views

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
)

func TestNewInstanceMetadataView(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)

	assert.NotNil(t, view)
	assert.Equal(t, "test-project", view.projectID)
	assert.Equal(t, "us-central1-a", view.zone)
	assert.Equal(t, "test-instance", view.instanceName)
	assert.True(t, view.loading)
	assert.False(t, view.editMode)
	assert.NotNil(t, view.editor)
}

func TestInstanceMetadataViewInit(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)

	cmd := view.Init()
	assert.NotNil(t, cmd)
}

func TestInstanceMetadataViewUpdateLoadedMsg(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.loading = true

	// Set size to initialize viewport properly
	ctx := &context.ProgramContext{
		ContentWidth:  100,
		ContentHeight: 30,
	}
	view.SetContext(ctx)

	instanceMeta := &gcp.InstanceMetadata{
		Items: map[string]string{
			"custom-key": "custom-value",
			"ssh-keys":   "user:ssh-rsa AAAABBBB user@host",
		},
		Fingerprint: "test-fingerprint",
	}

	projectMeta := &gcp.InstanceMetadata{
		Items: map[string]string{
			"ssh-keys": "project-user:ssh-rsa CCCCDDDD project@host",
		},
		Fingerprint: "test-project-fp",
	}

	msg := metadataLoadedMsg{
		instanceMetadata: instanceMeta,
		projectMetadata:  projectMeta,
	}

	view.Update(msg)

	assert.False(t, view.loading)
	assert.NotNil(t, view.instanceMetadata)
	assert.NotNil(t, view.projectMetadata)
	assert.Equal(t, "test-fingerprint", view.fingerprint)
	assert.Equal(t, "custom-value", view.instanceMetadata.Items["custom-key"])
}

func TestInstanceMetadataViewUpdateErrorMsg(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.loading = true

	testErr := errors.New("test error") //nolint:err113 // Test error
	msg := metadataErrorMsg{err: testErr}

	view.Update(msg)

	assert.False(t, view.loading)
	assert.Equal(t, testErr, view.err)
}

func TestInstanceMetadataViewUpdateSavedMsg(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.saving = true
	view.editMode = true

	msg := metadataSavedMsg{}
	cmd := view.Update(msg)

	assert.False(t, view.saving)
	assert.True(t, view.saveSuccess)
	assert.False(t, view.editMode)
	assert.NotNil(t, cmd) // Should trigger reload
}

func TestInstanceMetadataViewUpdateSaveErrorMsg(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.saving = true

	testErr := errors.New("save error") //nolint:err113 // Test error
	msg := metadataSaveErrorMsg{err: testErr}

	view.Update(msg)

	assert.False(t, view.saving)
	assert.Equal(t, testErr, view.err)
}

func TestInstanceMetadataViewEnterEditMode(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.instanceMetadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"custom-key": "custom-value",
			"ssh-keys":   "user:ssh-rsa AAAABBBB user@host",
		},
		Fingerprint: "test-fingerprint",
	}
	view.ready = true
	view.width = 100
	view.height = 30

	view.enterEditMode()

	assert.True(t, view.editMode)
	assert.False(t, view.saveSuccess)
	assert.Nil(t, view.err)

	// Editor content should only have custom metadata (no SSH keys)
	content := view.editor.GetContent()
	assert.Contains(t, content, "custom-key")
	assert.NotContains(t, content, "ssh-keys")
}

func TestInstanceMetadataViewExitEditMode(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.editMode = true

	view.exitEditMode()

	assert.False(t, view.editMode)
}

func TestInstanceMetadataViewGetCustomMetadata(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.instanceMetadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"custom-key":  "custom-value",
			"another-key": "another-value",
			"ssh-keys":    "user:ssh-rsa AAAABBBB user@host",
			"sshKeys":     "other format",
		},
		Fingerprint: "test-fingerprint",
	}

	custom := view.getCustomMetadata()

	assert.Len(t, custom, 2)
	assert.Equal(t, "custom-value", custom["custom-key"])
	assert.Equal(t, "another-value", custom["another-key"])
	assert.NotContains(t, custom, "ssh-keys")
	assert.NotContains(t, custom, "sshKeys")
}

func TestInstanceMetadataViewParseProjectSSHKeys(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.projectMetadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"ssh-keys": "user1:ssh-rsa AAAABBBB user1@host\nuser2:ssh-ed25519 CCCCDDDD user2@host",
		},
		Fingerprint: "test-project-fp",
	}

	keys := view.parseProjectSSHKeys()

	assert.Len(t, keys, 2)
	assert.Equal(t, "user1", keys[0].Username)
	assert.Equal(t, "ssh-rsa", keys[0].KeyType)
	assert.Equal(t, "user2", keys[1].Username)
	assert.Equal(t, "ssh-ed25519", keys[1].KeyType)
}

func TestInstanceMetadataViewParseInstanceSSHKeys(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.instanceMetadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"ssh-keys": "instance-user:ssh-rsa EEEEFFFF instance@host",
		},
		Fingerprint: "test-fingerprint",
	}

	keys := view.parseInstanceSSHKeys()

	assert.Len(t, keys, 1)
	assert.Equal(t, "instance-user", keys[0].Username)
	assert.Equal(t, "ssh-rsa", keys[0].KeyType)
}

func TestInstanceMetadataViewRenderLoading(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)

	output := view.renderLoading("Test message")

	assert.Contains(t, output, "Test message")
}

func TestInstanceMetadataViewViewLoadingState(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.loading = true

	output := view.View()

	assert.Contains(t, output, "Loading metadata")
}

func TestInstanceMetadataViewViewSavingState(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.loading = false
	view.saving = true

	output := view.View()

	assert.Contains(t, output, "Saving metadata")
}

func TestInstanceMetadataViewViewErrorState(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.loading = false
	view.err = errors.New("test error") //nolint:err113 // Test error

	output := view.View()

	assert.Contains(t, output, "Error")
	assert.Contains(t, output, "test error")
}

func TestInstanceMetadataViewRenderContent(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.instanceMetadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"custom-key": "custom-value",
			"ssh-keys":   "user:ssh-rsa AAAABBBB user@host",
		},
		Fingerprint: "test-fingerprint",
	}
	view.projectMetadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"ssh-keys": "project-user:ssh-rsa CCCCDDDD project@host",
		},
		Fingerprint: "test-project-fp",
	}
	view.ready = true
	view.width = 100
	view.height = 30
	view.viewport.Width = 100
	view.viewport.Height = 26

	content := view.renderContent()

	assert.Contains(t, content, "Instance Metadata")
	assert.Contains(t, content, "Custom Metadata")
	assert.Contains(t, content, "custom-key")
	assert.Contains(t, content, "custom-value")
	assert.Contains(t, content, "SSH Keys (Project-wide)")
	assert.Contains(t, content, "project-user")
	assert.Contains(t, content, "SSH Keys (Instance-specific)")
	assert.Contains(t, content, "user")
}

func TestInstanceMetadataViewRenderContentNoMetadata(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.instanceMetadata = &gcp.InstanceMetadata{
		Items:       map[string]string{},
		Fingerprint: "test-fingerprint",
	}
	view.projectMetadata = &gcp.InstanceMetadata{
		Items:       map[string]string{},
		Fingerprint: "test-project-fp",
	}
	view.ready = true
	view.width = 100
	view.height = 30
	view.viewport.Width = 100
	view.viewport.Height = 26

	content := view.renderContent()

	assert.Contains(t, content, "No custom metadata defined")
	assert.Contains(t, content, "No project-wide SSH keys defined")
	assert.Contains(t, content, "No instance-specific SSH keys defined")
}

func TestInstanceMetadataViewSetContext(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)

	ctx := &context.ProgramContext{
		ContentWidth:  100,
		ContentHeight: 30,
	}

	view.SetContext(ctx)

	assert.Equal(t, 100, view.width)
	assert.Equal(t, 30, view.height)
	assert.True(t, view.ready)
}

func TestInstanceMetadataViewUpdateKeyBindingsViewMode(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.loading = false
	view.ready = true
	view.editMode = false

	// Test edit key
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	cmd := view.Update(keyMsg)

	assert.True(t, view.editMode)
	assert.NotNil(t, cmd) // Should return focus command

	// Reset for refresh test
	view.editMode = false

	// Test refresh key
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	cmd = view.Update(keyMsg)

	assert.True(t, view.loading)
	assert.NotNil(t, cmd)
}

func TestInstanceMetadataViewUpdateKeyBindingsEditMode(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.loading = false
	view.editMode = true
	view.instanceMetadata = &gcp.InstanceMetadata{
		Items:       map[string]string{"key": "value"},
		Fingerprint: "test",
	}

	// Test escape key to cancel
	keyMsg := tea.KeyMsg{Type: tea.KeyEsc}
	view.Update(keyMsg)

	assert.False(t, view.editMode)
}

func TestInstanceMetadataViewRenderEditMode(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.editMode = true
	view.ready = true
	view.width = 100
	view.height = 30

	output := view.renderEditMode()

	assert.Contains(t, output, "ctrl+s: save")
	assert.Contains(t, output, "esc: cancel")
}

func TestInstanceMetadataViewRenderEditModeWithError(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.editMode = true
	view.ready = true
	view.width = 100
	view.height = 30
	view.err = errors.New("validation error") //nolint:err113 // Test error

	output := view.renderEditMode()

	assert.Contains(t, output, "Error")
	assert.Contains(t, output, "validation error")
}

func TestInstanceMetadataViewRenderViewMode(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.ready = true
	view.width = 100
	view.height = 30
	view.instanceMetadata = &gcp.InstanceMetadata{
		Items:       map[string]string{"key": "value"},
		Fingerprint: "test",
	}
	view.viewport.Width = 100
	view.viewport.Height = 26

	output := view.renderViewMode()

	assert.Contains(t, output, "↑/↓: scroll")
	assert.Contains(t, output, "e: edit")
	assert.Contains(t, output, "r: refresh")
}

func TestInstanceMetadataViewRenderViewModeWithSuccess(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.ready = true
	view.width = 100
	view.height = 30
	view.saveSuccess = true
	view.instanceMetadata = &gcp.InstanceMetadata{
		Items:       map[string]string{"key": "value"},
		Fingerprint: "test",
	}
	view.viewport.Width = 100
	view.viewport.Height = 26

	output := view.renderViewMode()

	assert.Contains(t, output, "Metadata saved successfully")
}

func TestInstanceMetadataViewUpdateViewportContent(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)
	view.ready = true
	view.width = 100
	view.height = 30
	view.instanceMetadata = &gcp.InstanceMetadata{
		Items:       map[string]string{"key": "value"},
		Fingerprint: "test",
	}
	view.viewport.Width = 100
	view.viewport.Height = 26

	// Should not panic
	view.updateViewportContent()

	// Viewport should have content
	assert.NotEmpty(t, view.viewport.View())
}

func TestInstanceMetadataViewLoadMetadataCmd(t *testing.T) {
	view := &InstanceMetadataView{
		computeClient: &gcp.ComputeClient{},
		projectID:     "test-project",
		zone:          "us-central1-a",
		instanceName:  "test-instance",
	}

	// We can't easily test the command execution without dependency injection
	// This test at least verifies the command is created
	cmd := view.loadMetadata()
	assert.NotNil(t, cmd)
}

func TestInstanceMetadataViewTruncateLongValues(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)

	longValue := strings.Repeat("a", 100)
	view.instanceMetadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"long-key": longValue,
		},
		Fingerprint: "test",
	}
	view.ready = true
	view.width = 100
	view.height = 30
	view.viewport.Width = 100
	view.viewport.Height = 26

	content := view.renderContent()

	// Should contain truncated value with ellipsis
	assert.Contains(t, content, "...")
}

func TestInstanceMetadataViewSSHKeyParsing(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewInstanceMetadataView("test-project", "us-central1-a", "test-instance", client)

	// Test with both ssh-keys formats
	view.instanceMetadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"ssh-keys": "user1:ssh-rsa AAAABBBBCCCCDDDDEEEEFFFFGGGG user1@example.com\nuser2:ssh-ed25519 HHHHIIIIJJJJKKKKLLLLMMMM user2@example.com",
		},
		Fingerprint: "test",
	}
	view.projectMetadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"ssh-keys": "admin:ssh-rsa NNNNOOOOppppqqqqrrrrssss admin@example.com",
		},
		Fingerprint: "test-project-fp",
	}
	view.ready = true
	view.width = 100
	view.height = 30
	view.viewport.Width = 100
	view.viewport.Height = 26

	content := view.renderContent()

	// Should show both project and instance SSH keys
	assert.Contains(t, content, "admin")
	assert.Contains(t, content, "user1")
	assert.Contains(t, content, "user2")
	assert.Contains(t, content, "ssh-rsa")
	assert.Contains(t, content, "ssh-ed25519")
}
