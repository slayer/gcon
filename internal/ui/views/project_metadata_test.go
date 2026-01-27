package views

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
)

func TestNewProjectMetadataView(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)

	assert.NotNil(t, view)
	assert.Equal(t, "test-project", view.projectID)
	assert.True(t, view.loading)
	assert.False(t, view.editMode)
	assert.False(t, view.showWarning)
	assert.NotNil(t, view.editor)
}

func TestProjectMetadataViewInit(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)

	cmd := view.Init()
	assert.NotNil(t, cmd)
}

func TestProjectMetadataViewUpdateLoadedMsg(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.loading = true

	// Set size to initialize viewport properly
	ctx := &context.ProgramContext{
		ContentWidth:  100,
		ContentHeight: 30,
	}
	view.SetContext(ctx)

	metadata := &gcp.InstanceMetadata{
		Items: map[string]string{
			"custom-key": "custom-value",
			"ssh-keys":   "admin:ssh-rsa AAAABBBB admin@host",
		},
		Fingerprint: "test-fingerprint",
	}

	msg := projectMetadataLoadedMsg{
		metadata: metadata,
	}

	view.Update(msg)

	assert.False(t, view.loading)
	assert.NotNil(t, view.metadata)
	assert.Equal(t, "test-fingerprint", view.fingerprint)
	assert.Equal(t, "custom-value", view.metadata.Items["custom-key"])
}

func TestProjectMetadataViewUpdateErrorMsg(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.loading = true

	testErr := errors.New("test error") //nolint:err113 // Test error
	msg := projectMetadataErrorMsg{err: testErr}

	view.Update(msg)

	assert.False(t, view.loading)
	assert.Equal(t, testErr, view.err)
}

func TestProjectMetadataViewUpdateSavedMsg(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.saving = true
	view.editMode = true
	view.showWarning = true

	msg := projectMetadataSavedMsg{}
	cmd := view.Update(msg)

	assert.False(t, view.saving)
	assert.True(t, view.saveSuccess)
	assert.False(t, view.editMode)
	assert.False(t, view.showWarning)
	assert.NotNil(t, cmd) // Should return load command
}

func TestProjectMetadataViewUpdateSaveErrorMsg(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.saving = true
	view.showWarning = true

	testErr := errors.New("save error") //nolint:err113 // Test error
	msg := projectMetadataSaveErrorMsg{err: testErr}

	view.Update(msg)

	assert.False(t, view.saving)
	assert.False(t, view.showWarning)
	assert.Equal(t, testErr, view.err)
}

func TestProjectMetadataViewEditModeTransition(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.loading = false
	view.ready = true
	view.metadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"key1": "value1",
		},
		Fingerprint: "test-fp",
	}

	// Set size
	ctx := &context.ProgramContext{
		ContentWidth:  100,
		ContentHeight: 30,
	}
	view.SetContext(ctx)

	// Press 'e' to enter edit mode
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	view.Update(msg)

	assert.True(t, view.editMode)
}

func TestProjectMetadataViewExitEditMode(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.editMode = true
	view.showWarning = false
	view.loading = false

	// Press Esc to exit edit mode
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	view.Update(msg)

	assert.False(t, view.editMode)
}

func TestProjectMetadataViewSaveWarning(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.editMode = true
	view.loading = false

	// Press Ctrl+S to trigger save (should show warning first)
	msg := tea.KeyMsg{Type: tea.KeyCtrlS}
	view.Update(msg)

	assert.True(t, view.showWarning)
	assert.True(t, view.editMode)
}

func TestProjectMetadataViewConfirmSave(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.showWarning = true
	view.editMode = true
	view.loading = false
	view.metadata = &gcp.InstanceMetadata{
		Items:       map[string]string{"key": "value"},
		Fingerprint: "test-fp",
	}
	view.fingerprint = "test-fp"
	view.editor.SetContent("key = value")

	// Press Ctrl+S in warning mode to confirm save
	msg := tea.KeyMsg{Type: tea.KeyCtrlS, Runes: []rune{19}}
	cmd := view.Update(msg)

	assert.True(t, view.saving)
	assert.NotNil(t, cmd)
}

func TestProjectMetadataViewCancelWarning(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.showWarning = true
	view.loading = false

	// Press Esc to cancel warning
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	view.Update(msg)

	assert.False(t, view.showWarning)
}

func TestProjectMetadataViewRefresh(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.loading = false
	view.err = errors.New("previous error") //nolint:err113 // Test error

	// Press 'r' to refresh
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	cmd := view.Update(msg)

	assert.True(t, view.loading)
	assert.Nil(t, view.err)
	assert.NotNil(t, cmd)
}

func TestProjectMetadataViewParseSSHKeys(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.metadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"ssh-keys": "user1:ssh-rsa AAAABBBB user1@host\nuser2:ssh-ed25519 CCCCDDDD user2@host",
		},
		Fingerprint: "test-fp",
	}

	keys := view.parseSSHKeys()

	assert.Len(t, keys, 2)
	assert.Equal(t, "user1", keys[0].Username)
	assert.Equal(t, "ssh-rsa", keys[0].KeyType)
	assert.Equal(t, "user2", keys[1].Username)
	assert.Equal(t, "ssh-ed25519", keys[1].KeyType)
}

func TestProjectMetadataViewParseSSHKeysEmpty(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.metadata = &gcp.InstanceMetadata{
		Items:       map[string]string{},
		Fingerprint: "test-fp",
	}

	keys := view.parseSSHKeys()

	assert.Len(t, keys, 0)
}

func TestProjectMetadataViewGetCustomMetadata(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.metadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"custom-key": "custom-value",
			"ssh-keys":   "user:ssh-rsa AAAABBBB user@host",
			"other-key":  "other-value",
		},
		Fingerprint: "test-fp",
	}

	customMeta := view.getCustomMetadata()

	assert.Len(t, customMeta, 2)
	assert.Equal(t, "custom-value", customMeta["custom-key"])
	assert.Equal(t, "other-value", customMeta["other-key"])
	_, hasSSHKeys := customMeta["ssh-keys"]
	assert.False(t, hasSSHKeys)
}

func TestProjectMetadataViewRenderContent(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.metadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"custom-key": "custom-value",
			"ssh-keys":   "admin:ssh-rsa AAAABBBB admin@host",
		},
		Fingerprint: "test-fp",
	}
	view.ready = true
	view.width = 100
	view.height = 30

	// Set size to initialize viewport
	ctx := &context.ProgramContext{
		ContentWidth:  100,
		ContentHeight: 30,
	}
	view.SetContext(ctx)

	content := view.renderContent()

	assert.NotEmpty(t, content)
	assert.Contains(t, content, "Project Metadata")
	assert.Contains(t, content, "test-project")
	assert.Contains(t, content, "SSH Keys")
	assert.Contains(t, content, "Custom Metadata")
	assert.Contains(t, content, "custom-key")
	assert.Contains(t, content, "admin")
}

func TestProjectMetadataViewRenderContentNoMetadata(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.metadata = &gcp.InstanceMetadata{
		Items:       map[string]string{},
		Fingerprint: "test-fp",
	}
	view.ready = true
	view.width = 100
	view.height = 30

	// Set size to initialize viewport
	ctx := &context.ProgramContext{
		ContentWidth:  100,
		ContentHeight: 30,
	}
	view.SetContext(ctx)

	content := view.renderContent()

	assert.NotEmpty(t, content)
	assert.Contains(t, content, "No project-wide SSH keys")
	assert.Contains(t, content, "No custom metadata")
}

func TestProjectMetadataViewViewLoading(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.loading = true

	output := view.View()

	assert.Contains(t, output, "Loading project metadata")
}

func TestProjectMetadataViewViewSaving(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.loading = false
	view.saving = true

	output := view.View()

	assert.Contains(t, output, "Saving project metadata")
}

func TestProjectMetadataViewViewError(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.loading = false
	view.err = errors.New("test error") //nolint:err113 // Test error

	output := view.View()

	assert.Contains(t, output, "Error")
	assert.Contains(t, output, "test error")
}

func TestProjectMetadataViewViewWarning(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.loading = false
	view.ready = true
	view.showWarning = true
	view.metadata = &gcp.InstanceMetadata{
		Items:       map[string]string{},
		Fingerprint: "test-fp",
	}

	output := view.View()

	assert.Contains(t, output, "WARNING")
	assert.Contains(t, output, "ALL instances")
	assert.Contains(t, output, "test-project")
}

func TestProjectMetadataViewSetContext(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)

	ctx := &context.ProgramContext{
		ContentWidth:  120,
		ContentHeight: 40,
	}
	view.SetContext(ctx)

	assert.Equal(t, 120, view.width)
	assert.Equal(t, 40, view.height)
	assert.NotNil(t, view.ctx)
}

func TestProjectMetadataViewKeysIgnoredDuringLoading(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.loading = true

	// Try to press 'e' during loading
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	view.Update(msg)

	// Should not enter edit mode
	assert.False(t, view.editMode)
}

func TestProjectMetadataViewKeysIgnoredDuringSaving(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)
	view.saving = true

	// Try to press 'e' during saving
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	view.Update(msg)

	// Should not enter edit mode
	assert.False(t, view.editMode)
}

func TestProjectMetadataViewSSHKeyParsing(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)

	// Test with ssh-keys format (use long keys to trigger truncation)
	longKey1 := strings.Repeat("A", 50) // 50 chars, will be truncated
	longKey2 := strings.Repeat("H", 50) // 50 chars, will be truncated
	view.metadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"ssh-keys": fmt.Sprintf("user1:ssh-rsa %s user1@example.com\nuser2:ssh-ed25519 %s user2@example.com", longKey1, longKey2),
		},
		Fingerprint: "test",
	}
	view.ready = true
	view.width = 100
	view.height = 30

	// Set size to initialize viewport
	ctx := &context.ProgramContext{
		ContentWidth:  100,
		ContentHeight: 30,
	}
	view.SetContext(ctx)

	content := view.renderContent()

	// Verify SSH keys are parsed and displayed
	assert.Contains(t, content, "user1")
	assert.Contains(t, content, "user2")
	assert.Contains(t, content, "ssh-rsa")
	assert.Contains(t, content, "ssh-ed25519")

	// Verify keys are truncated for display (full key should not appear)
	assert.NotContains(t, content, longKey1)
	assert.NotContains(t, content, longKey2)
	// But should contain the ellipsis from truncation
	assert.Contains(t, content, "...")
}

func TestProjectMetadataViewRenderContentLongValues(t *testing.T) {
	client := &gcp.ComputeClient{}
	view := NewProjectMetadataView("test-project", client)

	longValue := strings.Repeat("x", 100)
	view.metadata = &gcp.InstanceMetadata{
		Items: map[string]string{
			"long-key": longValue,
		},
		Fingerprint: "test-fp",
	}
	view.ready = true
	view.width = 100
	view.height = 30

	// Set size to initialize viewport
	ctx := &context.ProgramContext{
		ContentWidth:  100,
		ContentHeight: 30,
	}
	view.SetContext(ctx)

	content := view.renderContent()

	// Long values should be truncated with "..."
	assert.Contains(t, content, "long-key")
	assert.Contains(t, content, "...")
	// Full value should not be in output
	assert.NotContains(t, content, longValue)
}
