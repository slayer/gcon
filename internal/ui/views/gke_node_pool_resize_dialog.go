package views

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GKENodePoolResizeSubmitMsg is emitted when the user confirms the resize.
type GKENodePoolResizeSubmitMsg struct {
	PoolName         string
	Mode             GKENodePoolResizeMode
	NodeCount        int64 // manual mode
	AutoscaleEnabled bool  // autoscale mode
	MinNodes         int64 // autoscale mode
	MaxNodes         int64 // autoscale mode
}

// GKENodePoolResizeCanceledMsg is emitted when the user presses Esc.
type GKENodePoolResizeCanceledMsg struct{}

// focusTarget identifies which control is currently focused in autoscale mode.
type focusTarget int

const (
	focusToggle focusTarget = iota
	focusMin
	focusMax
	focusManual
)

// GKENodePoolResizeDialog is a dialog for resizing a node pool, supporting
// both manual (fixed count) and autoscale (min/max) modes.
type GKENodePoolResizeDialog struct {
	poolName      string
	mode          GKENodePoolResizeMode
	currentCount  int64
	currentMin    int64
	currentMax    int64
	currentAutoOn bool

	manualInput textinput.Model
	minInput    textinput.Model
	maxInput    textinput.Model
	autoEnabled bool   // user-edited autoscale-enable toggle
	focus       focusTarget
	err         string // validation error
}

// NewGKENodePoolResizeDialog creates a node-pool resize dialog.
// The inputs start empty; current values are shown as labels/placeholders.
func NewGKENodePoolResizeDialog(poolName string, currentCount int64, autoscaleEnabled bool, currentMin, currentMax int64) *GKENodePoolResizeDialog {
	bg := lipgloss.Color("#1E1E2E")

	// Manual input: empty so that the first key press yields only that digit.
	manual := textinput.New()
	manual.Placeholder = fmt.Sprintf("%d", currentCount)
	manual.CharLimit = 6
	manual.Width = 10
	manual.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED")).Background(bg)
	manual.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Background(bg)
	manual.PromptStyle = lipgloss.NewStyle().Background(bg)
	manual.Cursor.TextStyle = lipgloss.NewStyle().Background(bg)
	manual.Focus()

	// Min/max inputs for autoscale mode.
	makeInput := func(placeholder string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.CharLimit = 6
		ti.Width = 10
		ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E8EAED")).Background(bg)
		ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Background(bg)
		ti.PromptStyle = lipgloss.NewStyle().Background(bg)
		ti.Cursor.TextStyle = lipgloss.NewStyle().Background(bg)
		return ti
	}

	minPlaceholder := "0"
	if currentMin > 0 {
		minPlaceholder = fmt.Sprintf("%d", currentMin)
	}
	maxPlaceholder := "0"
	if currentMax > 0 {
		maxPlaceholder = fmt.Sprintf("%d", currentMax)
	}

	return &GKENodePoolResizeDialog{
		poolName:      poolName,
		mode:          GKENodePoolResizeManual,
		currentCount:  currentCount,
		currentMin:    currentMin,
		currentMax:    currentMax,
		currentAutoOn: autoscaleEnabled,
		manualInput:   manual,
		minInput:      makeInput(minPlaceholder),
		maxInput:      makeInput(maxPlaceholder),
		autoEnabled:   autoscaleEnabled,
		focus:         focusManual,
	}
}

// Mode returns the current resize mode (Manual or Autoscale).
func (d *GKENodePoolResizeDialog) Mode() GKENodePoolResizeMode { return d.mode }

// Init returns the initial blink command so the cursor blinks immediately.
func (d *GKENodePoolResizeDialog) Init() tea.Cmd { return textinput.Blink }

// HasTextInputFocused returns true when any text input is active (focused).
func (d *GKENodePoolResizeDialog) HasTextInputFocused() bool {
	return d.manualInput.Focused() || d.minInput.Focused() || d.maxInput.Focused()
}

// Update processes messages. Accepts tea.Msg (not just tea.KeyMsg) so that
// textinput.Blink commands propagate and the cursor blinks correctly.
func (d *GKENodePoolResizeDialog) Update(msg tea.Msg) (tea.Cmd, bool) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		return d.handleKey(keyMsg)
	}

	// Pass all non-key messages to the focused input for blink/cursor updates.
	return d.updateFocusedInput(msg), false
}

func (d *GKENodePoolResizeDialog) handleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "esc":
		return func() tea.Msg { return GKENodePoolResizeCanceledMsg{} }, true

	case "tab":
		return d.cycleMode(), true

	case "enter":
		cmd, consumed := d.submit()
		return cmd, consumed

	case "shift+tab":
		// In autoscale mode, cycle backward through inputs; ignore in manual.
		if d.mode == GKENodePoolResizeAutoscale {
			d.cycleFocusBackward()
			return nil, true
		}
		return nil, true
	}

	// Delegate character input to the focused input.
	cmd := d.updateFocusedInput(msg)
	if cmd != nil {
		return cmd, true
	}
	// Even with no cmd, mark as consumed so unknown keys don't bubble up.
	return nil, true
}

// cycleMode toggles between Manual and Autoscale, adjusting focus state.
func (d *GKENodePoolResizeDialog) cycleMode() tea.Cmd {
	if d.mode == GKENodePoolResizeManual {
		d.mode = GKENodePoolResizeAutoscale
		d.manualInput.Blur()
		// Start focus on the toggle in autoscale mode.
		d.focus = focusToggle
		d.minInput.Blur()
		d.maxInput.Blur()
	} else {
		d.mode = GKENodePoolResizeManual
		d.focus = focusManual
		d.manualInput.Focus()
		d.minInput.Blur()
		d.maxInput.Blur()
	}
	d.err = ""
	return textinput.Blink
}

// cycleFocusForward advances focus within autoscale mode: toggle → min → max → toggle.
func (d *GKENodePoolResizeDialog) cycleFocusForward() {
	switch d.focus {
	case focusToggle:
		d.focus = focusMin
		d.minInput.Focus()
	case focusMin:
		d.focus = focusMax
		d.minInput.Blur()
		d.maxInput.Focus()
	case focusMax:
		d.focus = focusToggle
		d.maxInput.Blur()
	}
}

// cycleFocusBackward reverses focus within autoscale mode.
func (d *GKENodePoolResizeDialog) cycleFocusBackward() {
	switch d.focus {
	case focusToggle:
		d.focus = focusMax
		d.maxInput.Focus()
	case focusMin:
		d.focus = focusToggle
		d.minInput.Blur()
	case focusMax:
		d.focus = focusMin
		d.maxInput.Blur()
		d.minInput.Focus()
	}
}

// updateFocusedInput forwards msg to whichever text input is active.
func (d *GKENodePoolResizeDialog) updateFocusedInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch d.mode {
	case GKENodePoolResizeManual:
		d.manualInput, cmd = d.manualInput.Update(msg)
	case GKENodePoolResizeAutoscale:
		switch d.focus {
		case focusMin:
			d.minInput, cmd = d.minInput.Update(msg)
		case focusMax:
			d.maxInput, cmd = d.maxInput.Update(msg)
		case focusToggle:
			// Toggle doesn't use textinput; handle space/enter via key handling.
			if keyMsg, ok := msg.(tea.KeyMsg); ok && (keyMsg.String() == " " || keyMsg.String() == "enter") {
				d.autoEnabled = !d.autoEnabled
			}
		}
	}
	return cmd
}

// submit validates and emits GKENodePoolResizeSubmitMsg, or sets d.err.
func (d *GKENodePoolResizeDialog) submit() (tea.Cmd, bool) {
	d.err = ""

	switch d.mode {
	case GKENodePoolResizeManual:
		raw := strings.TrimSpace(d.manualInput.Value())
		if raw == "" {
			d.err = "node count is required"
			return nil, true
		}
		count, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || count < 0 {
			d.err = "node count must be a non-negative integer"
			return nil, true
		}
		poolName := d.poolName
		return func() tea.Msg {
			return GKENodePoolResizeSubmitMsg{
				PoolName:  poolName,
				Mode:      GKENodePoolResizeManual,
				NodeCount: count,
			}
		}, true

	case GKENodePoolResizeAutoscale:
		minRaw := strings.TrimSpace(d.minInput.Value())
		maxRaw := strings.TrimSpace(d.maxInput.Value())
		if minRaw == "" || maxRaw == "" {
			d.err = "min and max node counts are required"
			return nil, true
		}
		minVal, err := strconv.ParseInt(minRaw, 10, 64)
		if err != nil || minVal < 0 {
			d.err = "min nodes must be a non-negative integer"
			return nil, true
		}
		maxVal, err := strconv.ParseInt(maxRaw, 10, 64)
		if err != nil || maxVal < 0 {
			d.err = "max nodes must be a non-negative integer"
			return nil, true
		}
		if minVal > maxVal {
			d.err = "min nodes must be ≤ max nodes"
			return nil, true
		}
		poolName := d.poolName
		autoEnabled := d.autoEnabled
		return func() tea.Msg {
			return GKENodePoolResizeSubmitMsg{
				PoolName:         poolName,
				Mode:             GKENodePoolResizeAutoscale,
				AutoscaleEnabled: autoEnabled,
				MinNodes:         minVal,
				MaxNodes:         maxVal,
			}
		}, true
	}

	return nil, false
}

// View renders the dialog.
func (d *GKENodePoolResizeDialog) View() string {
	dialogStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4285F4"))
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E8EAED"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	modeActiveStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#4285F4")).
		Padding(0, 1)
	modeInactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9AA0A6")).
		Padding(0, 1)
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	focusedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")).Bold(true)

	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Resize node pool: %s", d.poolName)))
	b.WriteString("\n\n")

	// Mode selector.
	manualLabel := "Manual"
	autoscaleLabel := "Autoscale"
	if d.mode == GKENodePoolResizeManual {
		b.WriteString(modeActiveStyle.Render(manualLabel))
		b.WriteString("  ")
		b.WriteString(modeInactiveStyle.Render(autoscaleLabel))
	} else {
		b.WriteString(modeInactiveStyle.Render(manualLabel))
		b.WriteString("  ")
		b.WriteString(modeActiveStyle.Render(autoscaleLabel))
	}
	b.WriteString("\n\n")

	switch d.mode {
	case GKENodePoolResizeManual:
		b.WriteString(labelStyle.Render(fmt.Sprintf("Current: %d nodes", d.currentCount)))
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("New count: "))
		b.WriteString(d.manualInput.View())
		b.WriteString("\n")

	case GKENodePoolResizeAutoscale:
		// Toggle.
		toggleFocused := d.focus == focusToggle
		toggleLabel := "Autoscaling enabled:"
		toggleVal := "No"
		if d.autoEnabled {
			toggleVal = "Yes"
		}
		if toggleFocused {
			b.WriteString(focusedStyle.Render("▶ "+toggleLabel) + " " + toggleVal)
		} else {
			b.WriteString(labelStyle.Render("  "+toggleLabel) + " " + toggleVal)
		}
		b.WriteString("\n")

		// Min.
		if d.currentMin > 0 {
			b.WriteString(labelStyle.Render(fmt.Sprintf("  Current min: %d", d.currentMin)))
			b.WriteString("\n")
		}
		minLabel := "  Min nodes: "
		if d.focus == focusMin {
			b.WriteString(focusedStyle.Render("▶ Min nodes: "))
		} else {
			b.WriteString(labelStyle.Render(minLabel))
		}
		b.WriteString(d.minInput.View())
		b.WriteString("\n")

		// Max.
		if d.currentMax > 0 {
			b.WriteString(labelStyle.Render(fmt.Sprintf("  Current max: %d", d.currentMax)))
			b.WriteString("\n")
		}
		maxLabel := "  Max nodes: "
		if d.focus == focusMax {
			b.WriteString(focusedStyle.Render("▶ Max nodes: "))
		} else {
			b.WriteString(labelStyle.Render(maxLabel))
		}
		b.WriteString(d.maxInput.View())
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if d.err != "" {
		b.WriteString(errorStyle.Render("  "+d.err))
		b.WriteString("\n\n")
	}

	b.WriteString(mutedStyle.Render("Tab: switch mode  Enter: submit  Esc: cancel"))

	return dialogStyle.Render(b.String())
}
