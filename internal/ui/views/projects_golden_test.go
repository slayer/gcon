package views

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
)

func TestGolden_ProjectsView_Loaded(t *testing.T) {
	view := NewProjectsView(nil)
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(projectsLoadedMsg{projects: testProjects()})

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_ProjectsView_Loading(t *testing.T) {
	view := NewProjectsView(nil)
	ctx := goldenContext()
	view.SetContext(ctx)
	// View starts in loading state — don't send loaded message

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_ProjectsView_Error(t *testing.T) {
	view := NewProjectsView(nil)
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(projectsErrorMsg{err: fmt.Errorf("permission denied: caller does not have resourcemanager.projects.list")})

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_ProjectsView_Empty(t *testing.T) {
	view := NewProjectsView(nil)
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(projectsLoadedMsg{projects: nil})

	golden.RequireEqual(t, []byte(view.View()))
}
