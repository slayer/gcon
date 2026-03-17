// Package context provides shared state that propagates to all views and components.
// Inspired by gh-dash's ProgramContext pattern for centralized state management.
package context

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TaskState represents the current state of an async task
type TaskState int

const (
	TaskRunning TaskState = iota
	TaskFinished
	TaskError
)

// Task tracks an async operation with visual feedback
type Task struct {
	ID           string
	Description  string // "Loading instances..."
	State        TaskState
	Error        error
	StartTime    time.Time
	FinishedTime *time.Time
}

// ProgramContext holds shared state propagated to all views and components.
// This centralizes dimensions, styles, and async task tracking.
type ProgramContext struct {
	// Terminal dimensions
	ScreenWidth  int
	ScreenHeight int

	// Content area dimensions (excluding sidebar when active)
	ContentWidth  int
	ContentHeight int

	// EmojiWidthBudget is the number of extra characters subtracted from
	// ContentWidth by renderWithSidebar to compensate for wide-emoji
	// miscounting. Views that produce lines at ContentWidth will have those
	// lines wrapped by lipgloss MaxWidth(ContentWidth - EmojiWidthBudget).
	// Views with text that fills the full width should subtract this value.
	EmojiWidthBudget int

	// Sidebar state
	SidebarActive bool
	SidebarWidth  int

	// Styles reference - all components use these for consistent theming
	Styles *Styles

	// Task tracking - callback to start async operations with visual feedback
	StartTask func(task Task) tea.Cmd

	// Active tasks map (managed by App).
	// NOTE: This map is accessed only from within the Bubble Tea event loop
	// (Update/View methods). Do not read or write Tasks from goroutines
	// outside the event loop without adding proper synchronization.
	Tasks map[string]Task

	// Current project context
	ProjectID string

	// Global error to display
	Error error
}

// New creates a new ProgramContext with default values
func New() *ProgramContext {
	styles := DefaultStyles()
	return &ProgramContext{
		Styles: &styles,
		Tasks:  make(map[string]Task),
	}
}

// SetDimensions updates all dimension-related fields
func (ctx *ProgramContext) SetDimensions(screenW, screenH, contentW, contentH int) {
	ctx.ScreenWidth = screenW
	ctx.ScreenHeight = screenH
	ctx.ContentWidth = contentW
	ctx.ContentHeight = contentH
}

// HasActiveTask returns true if any task is currently running
func (ctx *ProgramContext) HasActiveTask() bool {
	for _, task := range ctx.Tasks {
		if task.State == TaskRunning {
			return true
		}
	}
	return false
}

// ActiveTaskDescription returns the description of the first running task
func (ctx *ProgramContext) ActiveTaskDescription() string {
	for _, task := range ctx.Tasks {
		if task.State == TaskRunning {
			return task.Description
		}
	}
	return ""
}

// Styles holds all application styles organized by component.
// Centralizing styles ensures consistent theming across the app.
type Styles struct {
	// Color palette - GCP inspired
	Colors ColorPalette

	// Component-specific styles
	Common  CommonStyles
	Table   TableStyles
	Sidebar SidebarStyles
	Footer  FooterStyles
	Help    HelpStyles
}

// ColorPalette defines the GCP-inspired color scheme
type ColorPalette struct {
	Primary   lipgloss.Color // Google Blue
	Secondary lipgloss.Color // Google Green
	Warning   lipgloss.Color // Google Yellow
	Error     lipgloss.Color // Google Red
	Muted     lipgloss.Color // Gray
	Bg        lipgloss.Color // Dark background
	BgLight   lipgloss.Color // Lighter background
	Text      lipgloss.Color // Primary text
	TextMuted lipgloss.Color // Secondary text
}

// CommonStyles holds styles used across multiple components
type CommonStyles struct {
	Title         lipgloss.Style
	Subtitle      lipgloss.Style
	Selected      lipgloss.Style
	Normal        lipgloss.Style
	Muted         lipgloss.Style
	Error         lipgloss.Style
	Success       lipgloss.Style
	Warning       lipgloss.Style
	StatusRunning lipgloss.Style
	StatusStopped lipgloss.Style
	StatusPending lipgloss.Style
}

// TableStyles holds table-specific styles
type TableStyles struct {
	Header       lipgloss.Style
	Cell         lipgloss.Style
	SelectedCell lipgloss.Style
	StatusBar    lipgloss.Style
}

// SidebarStyles holds sidebar-specific styles
type SidebarStyles struct {
	Container    lipgloss.Style
	Item         lipgloss.Style
	SelectedItem lipgloss.Style
	Category     lipgloss.Style
	Border       lipgloss.Style
}

// FooterStyles holds footer-specific styles
type FooterStyles struct {
	Container lipgloss.Style
	Text      lipgloss.Style
	Key       lipgloss.Style
	Separator lipgloss.Style
	TaskInfo  lipgloss.Style
}

// HelpStyles holds help display styles
type HelpStyles struct {
	Key  lipgloss.Style
	Desc lipgloss.Style
}

// DefaultStyles returns the default GCP-themed styles
func DefaultStyles() Styles {
	colors := ColorPalette{
		Primary:   lipgloss.Color("#4285F4"), // Google Blue
		Secondary: lipgloss.Color("#34A853"), // Google Green
		Warning:   lipgloss.Color("#FBBC05"), // Google Yellow
		Error:     lipgloss.Color("#EA4335"), // Google Red
		Muted:     lipgloss.Color("#9AA0A6"), // Gray
		Bg:        lipgloss.Color("#202124"), // Dark background
		BgLight:   lipgloss.Color("#303134"), // Lighter background
		Text:      lipgloss.Color("#FFFFFF"), // Primary text
		TextMuted: lipgloss.Color("#9AA0A6"), // Secondary text
	}

	return Styles{
		Colors: colors,
		Common: CommonStyles{
			Title: lipgloss.NewStyle().
				Bold(true).
				Foreground(colors.Primary),
			Subtitle: lipgloss.NewStyle().
				Foreground(colors.Muted),
			Selected: lipgloss.NewStyle().
				Bold(true).
				Foreground(colors.Text).
				Background(colors.Primary).
				Padding(0, 1),
			Normal: lipgloss.NewStyle().
				Foreground(colors.Text),
			Muted: lipgloss.NewStyle().
				Foreground(colors.Muted),
			Error: lipgloss.NewStyle().
				Foreground(colors.Error).
				Bold(true),
			Success: lipgloss.NewStyle().
				Foreground(colors.Secondary),
			Warning: lipgloss.NewStyle().
				Foreground(colors.Warning),
			StatusRunning: lipgloss.NewStyle().
				Foreground(colors.Secondary),
			StatusStopped: lipgloss.NewStyle().
				Foreground(colors.Error),
			StatusPending: lipgloss.NewStyle().
				Foreground(colors.Warning),
		},
		Table: TableStyles{
			Header: lipgloss.NewStyle().
				Bold(true).
				Foreground(colors.Primary).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(colors.Muted),
			Cell: lipgloss.NewStyle().
				Padding(0, 1),
			SelectedCell: lipgloss.NewStyle().
				Padding(0, 1).
				Background(colors.Primary).
				Foreground(colors.Text),
			StatusBar: lipgloss.NewStyle().
				Foreground(colors.Muted),
		},
		Sidebar: SidebarStyles{
			Container: lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderRight(true).
				BorderForeground(colors.Muted),
			Item: lipgloss.NewStyle().
				Padding(0, 1),
			SelectedItem: lipgloss.NewStyle().
				Padding(0, 1).
				Background(colors.Primary).
				Foreground(colors.Text),
			Category: lipgloss.NewStyle().
				Bold(true).
				Foreground(colors.Primary),
			Border: lipgloss.NewStyle().
				Foreground(colors.Muted),
		},
		Footer: FooterStyles{
			Container: lipgloss.NewStyle().
				Background(colors.BgLight),
			Text: lipgloss.NewStyle().
				Foreground(colors.Muted),
			Key: lipgloss.NewStyle().
				Foreground(colors.Text).
				Bold(true),
			Separator: lipgloss.NewStyle().
				Foreground(colors.Muted),
			TaskInfo: lipgloss.NewStyle().
				Foreground(colors.Primary).
				Italic(true),
		},
		Help: HelpStyles{
			Key: lipgloss.NewStyle().
				Foreground(colors.Text).
				Bold(true),
			Desc: lipgloss.NewStyle().
				Foreground(colors.Muted),
		},
	}
}
