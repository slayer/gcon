package ui

import "github.com/slayer/gcon/internal/gcp"

// Message types for async operations

// ProjectsLoadedMsg is sent when projects are fetched from GCP
type ProjectsLoadedMsg struct {
	Projects []gcp.Project
}

// ProjectSelectedMsg is sent when a project is selected
type ProjectSelectedMsg struct {
	Project gcp.Project
}

// ErrorMsg is sent when an error occurs
type ErrorMsg struct {
	Err     error
	Context string // Additional context about where the error occurred
}

// LoadingMsg indicates a loading state change
type LoadingMsg struct {
	Loading bool
	Message string
}

// RefreshMsg triggers a refresh of the current view
type RefreshMsg struct{}

// InitialProjectLoadedMsg is sent when the initial project (from config/flag) is loaded
type InitialProjectLoadedMsg struct {
	Project gcp.Project
}

// InitialProjectErrorMsg is sent when loading initial project fails
type InitialProjectErrorMsg struct {
	Err       error
	ProjectID string // The project ID that failed to load
}

// SidebarNavigateMsg is sent when a sidebar menu item is selected
type SidebarNavigateMsg struct {
	ViewType ViewType // Target view to navigate to
	ItemID   string   // Menu item identifier
}
