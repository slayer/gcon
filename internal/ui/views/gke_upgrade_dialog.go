package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GKEUpgradeCanceledMsg is emitted when the user presses Esc.
type GKEUpgradeCanceledMsg struct{}

// GKEUpgradeDialog is the shared version picker for control-plane and
// node-pool upgrades. The submit closure decides which message type
// to emit, so the same component drives both flows.
type GKEUpgradeDialog struct {
	title          string
	currentVersion string
	versions       []string // newest-first
	cursor         int
	submit         func(version string) tea.Msg
}

// NewGKEUpgradeDialog creates a version-picker dialog. versions should be
// newest-first (as returned by GetServerConfig). The submit closure wraps
// the chosen version in whatever message type the caller needs.
func NewGKEUpgradeDialog(title, current string, versions []string, submit func(string) tea.Msg) *GKEUpgradeDialog {
	return &GKEUpgradeDialog{
		title:          title,
		currentVersion: current,
		versions:       versions,
		submit:         submit,
	}
}

// Init satisfies the Bubble Tea convention; no initial commands needed.
func (d *GKEUpgradeDialog) Init() tea.Cmd { return nil }

// HasTextInputFocused — no text inputs in this dialog. Returned to
// satisfy the parent's gating; always false.
func (d *GKEUpgradeDialog) HasTextInputFocused() bool { return false }

// Update handles keyboard input. Returns (cmd, consumed).
func (d *GKEUpgradeDialog) Update(msg tea.Msg) (tea.Cmd, bool) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}

	switch keyMsg.String() {
	case "esc":
		return func() tea.Msg { return GKEUpgradeCanceledMsg{} }, true

	case "j", "down":
		if d.cursor+1 < len(d.versions) {
			d.cursor++
		}
		return nil, false

	case "k", "up":
		if d.cursor > 0 {
			d.cursor--
		}
		return nil, false

	case "enter":
		if d.cursor < 0 || d.cursor >= len(d.versions) {
			return nil, false
		}

		v := d.versions[d.cursor]

		return func() tea.Msg { return d.submit(v) }, true
	}

	return nil, false
}

// View renders the dialog box.
func (d *GKEUpgradeDialog) View() string {
	style := lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder())
	titleStyle := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	highlight := lipgloss.NewStyle().Background(lipgloss.Color("#4285F4")).Foreground(lipgloss.Color("#FFFFFF"))

	var b strings.Builder

	b.WriteString(titleStyle.Render(d.title))
	b.WriteString("\n\n")
	b.WriteString(muted.Render("Pick a version (j/k to move, Enter to submit, Esc to cancel)"))
	b.WriteString("\n\n")

	for i, v := range d.versions {
		label := v
		if v == d.currentVersion {
			label = v + " (current)"
		}

		if i == d.cursor {
			b.WriteString(highlight.Render("▶ " + label))
		} else {
			b.WriteString("  " + label)
		}

		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(muted.Render(fmt.Sprintf("Note: %s is a multi-minute operation.", strings.ToLower(d.title))))

	return style.Render(b.String())
}
