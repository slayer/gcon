// Package diff provides a component for displaying before/after comparisons
// with confirmation dialog for changes.
package diff

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Colors for diff display
var (
	colorAdded     = lipgloss.Color("#34A853") // Green for additions
	colorRemoved   = lipgloss.Color("#EA4335") // Red for removals
	colorUnchanged = lipgloss.Color("#9AA0A6") // Gray for unchanged
	colorMuted     = lipgloss.Color("#9AA0A6")
	colorPrimary   = lipgloss.Color("#4285F4")
	colorBgLight   = lipgloss.Color("#303134")
)

// Field represents a single field in the diff
type Field struct {
	Label    string
	OldValue string
	NewValue string
}

// IsChanged returns true if the field value has changed
func (f Field) IsChanged() bool {
	return f.OldValue != f.NewValue
}

// ConfirmMsg is sent when user confirms the changes
type ConfirmMsg struct{}

// CancelMsg is sent when user cancels the changes
type CancelMsg struct{}

// Viewer displays a diff with confirmation buttons
type Viewer struct {
	title      string
	fields     []Field
	warnings   []string
	focusedYes bool
	width      int
	height     int
	keys       keyMap
	styles     styles
}

type keyMap struct {
	Left    key.Binding
	Right   key.Binding
	Tab     key.Binding
	Confirm key.Binding
	Cancel  key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "yes"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "no"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter", "y"),
			key.WithHelp("enter/y", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc", "n"),
			key.WithHelp("esc/n", "cancel"),
		),
	}
}

type styles struct {
	Title          lipgloss.Style
	Container      lipgloss.Style
	Label          lipgloss.Style
	Added          lipgloss.Style
	Removed        lipgloss.Style
	Unchanged      lipgloss.Style
	Warning        lipgloss.Style
	ButtonActive   lipgloss.Style
	ButtonInactive lipgloss.Style
	Help           lipgloss.Style
	Divider        lipgloss.Style
}

func defaultStyles() styles {
	return styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1),
		Container: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2),
		Label: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			MarginTop(1),
		Added: lipgloss.NewStyle().
			Foreground(colorAdded),
		Removed: lipgloss.NewStyle().
			Foreground(colorRemoved).
			Strikethrough(true),
		Unchanged: lipgloss.NewStyle().
			Foreground(colorUnchanged),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBC05")),
		ButtonActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPrimary).
			Padding(0, 2),
		ButtonInactive: lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(colorBgLight).
			Padding(0, 2),
		Help: lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1),
		Divider: lipgloss.NewStyle().
			Foreground(colorMuted),
	}
}

// New creates a new diff viewer
func New(title string, fields []Field) *Viewer {
	return &Viewer{
		title:      title,
		fields:     fields,
		focusedYes: true,
		keys:       defaultKeyMap(),
		styles:     defaultStyles(),
	}
}

// SetWarnings sets warning messages to display
func (v *Viewer) SetWarnings(warnings []string) {
	v.warnings = warnings
}

// SetSize sets the component dimensions
func (v *Viewer) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// HasChanges returns true if any field has changed
func (v *Viewer) HasChanges() bool {
	for _, f := range v.fields {
		if f.IsChanged() {
			return true
		}
	}
	return false
}

// Update handles user input
func (v *Viewer) Update(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch {
	case key.Matches(keyMsg, v.keys.Left):
		v.focusedYes = true
		return nil

	case key.Matches(keyMsg, v.keys.Right):
		v.focusedYes = false
		return nil

	case key.Matches(keyMsg, v.keys.Tab):
		v.focusedYes = !v.focusedYes
		return nil

	case key.Matches(keyMsg, v.keys.Confirm):
		if v.focusedYes {
			return func() tea.Msg { return ConfirmMsg{} }
		}
		return func() tea.Msg { return CancelMsg{} }

	case key.Matches(keyMsg, v.keys.Cancel):
		return func() tea.Msg { return CancelMsg{} }
	}

	// Direct key shortcuts
	switch keyMsg.String() {
	case "y", "Y":
		return func() tea.Msg { return ConfirmMsg{} }
	case "n", "N":
		return func() tea.Msg { return CancelMsg{} }
	}

	return nil
}

// View renders the diff viewer
func (v *Viewer) View() string {
	var b strings.Builder

	// Title
	b.WriteString(v.styles.Title.Render(v.title))
	b.WriteString("\n\n")

	// Fields with changes
	hasChanges := false
	for _, field := range v.fields {
		if field.IsChanged() {
			hasChanges = true
			b.WriteString(v.styles.Label.Render(field.Label))
			b.WriteString("\n")

			// Show removed value
			if field.OldValue != "" {
				b.WriteString("  ")
				b.WriteString(v.styles.Removed.Render("- " + field.OldValue))
				b.WriteString("\n")
			}

			// Show added value
			if field.NewValue != "" {
				b.WriteString("  ")
				b.WriteString(v.styles.Added.Render("+ " + field.NewValue))
				b.WriteString("\n")
			} else if field.OldValue != "" {
				// Value was removed entirely
				b.WriteString("  ")
				b.WriteString(v.styles.Removed.Render("(removed)"))
				b.WriteString("\n")
			}
		}
	}

	// Show unchanged fields in muted style
	unchangedCount := 0
	for _, field := range v.fields {
		if !field.IsChanged() {
			unchangedCount++
		}
	}
	if unchangedCount > 0 {
		b.WriteString("\n")
		b.WriteString(v.styles.Unchanged.Render(
			strings.Repeat("─", 20) + " unchanged " + strings.Repeat("─", 20),
		))
		b.WriteString("\n")
		for _, field := range v.fields {
			if !field.IsChanged() && field.OldValue != "" {
				b.WriteString("  ")
				b.WriteString(v.styles.Unchanged.Render(field.Label + ": " + field.OldValue))
				b.WriteString("\n")
			}
		}
	}

	// Warnings
	if len(v.warnings) > 0 {
		b.WriteString("\n")
		for _, warning := range v.warnings {
			b.WriteString(v.styles.Warning.Render("⚠ " + warning))
			b.WriteString("\n")
		}
	}

	// Divider
	b.WriteString("\n")
	b.WriteString(v.styles.Divider.Render(strings.Repeat("─", 40)))
	b.WriteString("\n\n")

	// Buttons
	if !hasChanges {
		b.WriteString(v.styles.Unchanged.Render("No changes to apply"))
		b.WriteString("\n\n")
	}

	yesBtn := v.styles.ButtonInactive.Render("Yes, apply changes")
	noBtn := v.styles.ButtonInactive.Render("No, go back")

	if v.focusedYes {
		yesBtn = v.styles.ButtonActive.Render("Yes, apply changes")
	} else {
		noBtn = v.styles.ButtonActive.Render("No, go back")
	}

	b.WriteString(yesBtn)
	b.WriteString("  ")
	b.WriteString(noBtn)
	b.WriteString("\n\n")

	// Help
	b.WriteString(v.styles.Help.Render("←/→:select  enter:confirm  esc:cancel  y/n:quick"))

	// Container width based on provided width or default
	containerWidth := v.width
	if containerWidth < 50 {
		containerWidth = 50
	}

	return v.styles.Container.Width(containerWidth).Render(b.String())
}

// IsFocusedYes returns true if Yes button is focused
func (v *Viewer) IsFocusedYes() bool {
	return v.focusedYes
}
