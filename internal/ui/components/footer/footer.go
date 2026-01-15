// Package footer provides an enhanced footer component with dynamic content.
// Inspired by gh-dash's footer pattern with dynamic spacing and task indicators.
package footer

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/context"
)

// Model represents the footer component
type Model struct {
	ctx *context.ProgramContext

	// Dynamic sections
	leftSection  string // e.g., "3 instances"
	rightSection string // e.g., task progress

	// Help text
	helpText string

	// State
	showConfirmQuit bool
	showHelp        bool
	width           int
}

// New creates a new footer model
func New(ctx *context.ProgramContext) Model {
	return Model{
		ctx:      ctx,
		helpText: "? help • q quit",
	}
}

// SetContext updates the program context reference
func (m *Model) SetContext(ctx *context.ProgramContext) {
	m.ctx = ctx
}

// SetWidth updates the footer width
func (m *Model) SetWidth(width int) {
	m.width = width
}

// SetLeftSection sets the left content (e.g., resource counts)
func (m *Model) SetLeftSection(content string) {
	m.leftSection = content
}

// SetRightSection sets the right content (e.g., task status)
func (m *Model) SetRightSection(content string) {
	m.rightSection = content
}

// SetHelpText sets the help hint text
func (m *Model) SetHelpText(text string) {
	m.helpText = text
}

// SetShowConfirmQuit enables/disables quit confirmation mode
func (m *Model) SetShowConfirmQuit(show bool) {
	m.showConfirmQuit = show
}

// SetShowHelp enables/disables full help display
func (m *Model) SetShowHelp(show bool) {
	m.showHelp = show
}

// View renders the footer
func (m Model) View() string {
	if m.width == 0 || m.ctx == nil {
		return ""
	}

	// Confirm quit mode takes over the footer
	if m.showConfirmQuit {
		return m.renderConfirmQuit()
	}

	// Build footer sections
	left := m.renderLeftSection()
	right := m.renderRightSection()
	help := m.renderHelpIndicator()

	// Calculate spacing to fill remaining width
	usedWidth := lipgloss.Width(left) + lipgloss.Width(right) + lipgloss.Width(help)
	spacingWidth := m.width - usedWidth
	if spacingWidth < 0 {
		spacingWidth = 0
	}

	spacing := strings.Repeat(" ", spacingWidth)

	// Join all sections horizontally
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		spacing,
		right,
		help,
	)
}

// renderLeftSection renders the left side content
func (m Model) renderLeftSection() string {
	if m.leftSection == "" {
		return ""
	}

	style := lipgloss.NewStyle().
		Foreground(m.ctx.Styles.Colors.Muted)

	return style.Render(m.leftSection)
}

// renderRightSection renders the right side content (task status)
func (m Model) renderRightSection() string {
	// Show active task if any
	if m.ctx.HasActiveTask() {
		return m.renderActiveTask()
	}

	if m.rightSection != "" {
		style := lipgloss.NewStyle().
			Foreground(m.ctx.Styles.Colors.Muted)
		return style.Render(m.rightSection)
	}

	return ""
}

// renderActiveTask shows the currently running task with elapsed time
func (m Model) renderActiveTask() string {
	for _, task := range m.ctx.Tasks {
		if task.State == context.TaskRunning {
			elapsed := time.Since(task.StartTime).Round(time.Second)

			taskStyle := lipgloss.NewStyle().
				Foreground(m.ctx.Styles.Colors.Primary).
				Italic(true)

			timeStyle := lipgloss.NewStyle().
				Foreground(m.ctx.Styles.Colors.Muted)

			return fmt.Sprintf("%s %s",
				taskStyle.Render(task.Description),
				timeStyle.Render(fmt.Sprintf("(%s)", elapsed)))
		}
	}
	return ""
}

// renderHelpIndicator renders the help hint
func (m Model) renderHelpIndicator() string {
	style := lipgloss.NewStyle().
		Foreground(m.ctx.Styles.Colors.Muted)

	return style.Render(" • " + m.helpText)
}

// renderConfirmQuit renders the quit confirmation prompt
func (m Model) renderConfirmQuit() string {
	style := lipgloss.NewStyle().
		Foreground(m.ctx.Styles.Colors.Warning).
		Bold(true)

	return style.Render("Really quit? (y/enter to confirm, any other key to cancel)")
}

// FormatResourceCount formats a resource count for display
func FormatResourceCount(resourceType string, count int) string {
	if count == 1 {
		// Singularize common resource types
		singular := resourceType
		if strings.HasSuffix(resourceType, "s") {
			singular = resourceType[:len(resourceType)-1]
		}
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, resourceType)
}

// FormatLastRefresh formats a timestamp for display
func FormatLastRefresh(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	elapsed := time.Since(t).Round(time.Second)
	if elapsed < time.Minute {
		return "just now"
	}
	if elapsed < time.Hour {
		mins := int(elapsed.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	}
	return t.Format("15:04")
}
