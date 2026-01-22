package projectselector

import "github.com/slayer/gcon/internal/gcp"

// ProjectSelectedMsg is sent when a project is selected from the modal
type ProjectSelectedMsg struct {
	Project gcp.Project
}

// ProjectSelectorCanceledMsg is sent when the user cancels the project selector
type ProjectSelectorCanceledMsg struct{}

// Internal messages

type projectsLoadedMsg struct {
	projects []gcp.Project
}

type projectsErrorMsg struct {
	err error
}
