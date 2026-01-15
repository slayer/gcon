// Package views implements the view layer for the gcon TUI.
// All views implement the View interface for consistent context propagation.
package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/ui/context"
)

// View defines the interface that all gcon views must implement.
// This enables consistent context propagation and dimension management.
type View interface {
	// Init initializes the view and returns any startup commands
	Init() tea.Cmd

	// Update handles messages and returns any resulting commands
	Update(msg tea.Msg) tea.Cmd

	// View renders the view to a string
	View() string

	// SetContext updates the view with the shared program context.
	// Views should read dimensions from ctx.ContentWidth and ctx.ContentHeight.
	SetContext(ctx *context.ProgramContext)
}
