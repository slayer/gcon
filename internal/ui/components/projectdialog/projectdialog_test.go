package projectdialog

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	dialog := New(nil)

	assert.NotNil(t, dialog)
	assert.True(t, dialog.loading)
	assert.True(t, dialog.canCancel)
	assert.Equal(t, 0, dialog.cursor)
}

func TestSetCanCancel(t *testing.T) {
	dialog := New(nil)

	// Default is true
	assert.True(t, dialog.canCancel)

	dialog.SetCanCancel(false)
	assert.False(t, dialog.canCancel)

	dialog.SetCanCancel(true)
	assert.True(t, dialog.canCancel)
}

func TestSetCurrentProject(t *testing.T) {
	dialog := New(nil)

	dialog.SetCurrentProject("my-project")
	assert.Equal(t, "my-project", dialog.currentProjectID)

	dialog.SetCurrentProject("")
	assert.Equal(t, "", dialog.currentProjectID)
}

func TestFilterProjects(t *testing.T) {
	dialog := New(nil)
	dialog.loading = false
	dialog.projects = []gcp.Project{
		{ID: "project-a", Name: "Alpha Project"},
		{ID: "project-b", Name: "Beta Project"},
		{ID: "project-c", Name: "Gamma Project"},
	}

	// No filter - all projects
	dialog.filterProjects()
	assert.Len(t, dialog.filtered, 3)

	// Filter by name
	dialog.input.SetValue("alpha")
	dialog.filterProjects()
	assert.Len(t, dialog.filtered, 1)
	assert.Equal(t, "project-a", dialog.filtered[0].ID)

	// Filter by ID
	dialog.input.SetValue("project-b")
	dialog.filterProjects()
	assert.Len(t, dialog.filtered, 1)
	assert.Equal(t, "project-b", dialog.filtered[0].ID)

	// Case insensitive
	dialog.input.SetValue("GAMMA")
	dialog.filterProjects()
	assert.Len(t, dialog.filtered, 1)
	assert.Equal(t, "project-c", dialog.filtered[0].ID)

	// No match
	dialog.input.SetValue("nonexistent")
	dialog.filterProjects()
	assert.Len(t, dialog.filtered, 0)
}

func TestMoveCursor(t *testing.T) {
	dialog := New(nil)
	dialog.loading = false
	dialog.projects = []gcp.Project{
		{ID: "project-a", Name: "Alpha"},
		{ID: "project-b", Name: "Beta"},
		{ID: "project-c", Name: "Gamma"},
	}
	dialog.filterProjects()

	// Start at 0
	assert.Equal(t, 0, dialog.cursor)

	// Move down
	dialog.moveCursor(1)
	assert.Equal(t, 1, dialog.cursor)

	dialog.moveCursor(1)
	assert.Equal(t, 2, dialog.cursor)

	// Can't go past end
	dialog.moveCursor(1)
	assert.Equal(t, 2, dialog.cursor)

	// Move up
	dialog.moveCursor(-1)
	assert.Equal(t, 1, dialog.cursor)

	dialog.moveCursor(-1)
	assert.Equal(t, 0, dialog.cursor)

	// Can't go negative
	dialog.moveCursor(-1)
	assert.Equal(t, 0, dialog.cursor)
}

func TestViewLoading(t *testing.T) {
	dialog := New(nil)
	dialog.SetSize(80, 24)

	view := dialog.View()
	assert.Contains(t, view, "Loading projects")
}

func TestViewError(t *testing.T) {
	dialog := New(nil)
	dialog.SetSize(80, 24)
	dialog.loading = false
	dialog.err = assert.AnError

	view := dialog.View()
	assert.Contains(t, view, "Error:")
	assert.Contains(t, view, "r: retry")
}

func TestViewWithProjects(t *testing.T) {
	dialog := New(nil)
	dialog.SetSize(80, 24)
	dialog.loading = false
	dialog.projects = []gcp.Project{
		{ID: "project-a", Name: "Alpha Project", State: "ACTIVE"},
		{ID: "project-b", Name: "Beta Project", State: "ACTIVE"},
	}
	dialog.filterProjects()

	view := dialog.View()
	assert.Contains(t, view, "Alpha Project")
	assert.Contains(t, view, "Beta Project")
	assert.Contains(t, view, "enter: select")
}

func TestViewCurrentProjectMarked(t *testing.T) {
	dialog := New(nil)
	dialog.SetSize(80, 24)
	dialog.loading = false
	dialog.currentProjectID = "project-a"
	dialog.projects = []gcp.Project{
		{ID: "project-a", Name: "Alpha Project", State: "ACTIVE"},
		{ID: "project-b", Name: "Beta Project", State: "ACTIVE"},
	}
	dialog.filterProjects()

	view := dialog.View()
	// Should have checkmark for current project
	assert.Contains(t, view, "✓")
}

func TestUpdateKeyNavigation(t *testing.T) {
	dialog := New(nil)
	dialog.loading = false
	dialog.projects = []gcp.Project{
		{ID: "project-a", Name: "Alpha"},
		{ID: "project-b", Name: "Beta"},
	}
	dialog.filterProjects()

	// Down key
	dialog.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, dialog.cursor)

	// Up key
	dialog.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, dialog.cursor)

	// k key (vim style)
	dialog.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, dialog.cursor)
}

func TestUpdateEnterSelectsProject(t *testing.T) {
	dialog := New(nil)
	dialog.loading = false
	dialog.projects = []gcp.Project{
		{ID: "project-a", Name: "Alpha"},
		{ID: "project-b", Name: "Beta"},
	}
	dialog.filterProjects()
	dialog.cursor = 1

	cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd)

	// Execute the command and check the message
	msg := cmd()
	selectedMsg, ok := msg.(ProjectDialogSelectedMsg)
	assert.True(t, ok)
	assert.Equal(t, "project-b", selectedMsg.Project.ID)
}

func TestUpdateEscCloses(t *testing.T) {
	dialog := New(nil)
	dialog.loading = false
	dialog.canCancel = true
	dialog.projects = []gcp.Project{
		{ID: "project-a", Name: "Alpha"},
	}
	dialog.filterProjects()

	cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.NotNil(t, cmd)

	// Execute the command and check the message
	msg := cmd()
	_, ok := msg.(ProjectDialogClosedMsg)
	assert.True(t, ok)
}

func TestUpdateEscBlockedWhenCannotCancel(t *testing.T) {
	dialog := New(nil)
	dialog.loading = false
	dialog.canCancel = false
	dialog.projects = []gcp.Project{
		{ID: "project-a", Name: "Alpha"},
	}
	dialog.filterProjects()

	cmd := dialog.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, cmd)
}

func TestReset(t *testing.T) {
	dialog := New(nil)
	dialog.loading = false
	dialog.projects = []gcp.Project{
		{ID: "project-a", Name: "Alpha"},
		{ID: "project-b", Name: "Beta"},
	}
	dialog.filterProjects()
	dialog.cursor = 1
	dialog.input.SetValue("test")
	dialog.filterMode = true

	dialog.Reset()

	assert.Equal(t, 0, dialog.cursor)
	assert.Equal(t, "", dialog.input.Value())
	assert.False(t, dialog.filterMode)
}
