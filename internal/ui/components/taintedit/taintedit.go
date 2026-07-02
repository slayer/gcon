// Package taintedit provides a component for editing Kubernetes node-pool taints.
// Each taint has a Key, Value, and Effect (NO_SCHEDULE / PREFER_NO_SCHEDULE / NO_EXECUTE).
package taintedit

import (
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// Effect constants — match GCP enum values for node taints.
const (
	EffectNoSchedule       = "NO_SCHEDULE"
	EffectPreferNoSchedule = "PREFER_NO_SCHEDULE"
	EffectNoExecute        = "NO_EXECUTE"
)

// allEffects is the ordered cycle for Space key.
var allEffects = []string{EffectNoSchedule, EffectPreferNoSchedule, EffectNoExecute}

// k8s taint key: optional prefix (subdomain/name) + name segment.
// Prefix: lowercase alphanumerics, hyphens, and dots; starts/ends with alphanumeric.
// Name: alphanumerics, hyphens, underscores, dots; starts/ends with alphanumeric.
var taintKeyPattern = regexp.MustCompile(
	`^([a-z0-9]([-a-z0-9.]*[a-z0-9])?/)?[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`,
)

// k8s taint value: empty OR alphanumeric start/end with hyphens/underscores/dots inside.
var taintValuePattern = regexp.MustCompile(
	`^$|^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`,
)

// --- Colors (Google palette) ------------------------------------------------

var (
	colorPrimary = lipgloss.Color("#4285F4")
	colorError   = lipgloss.Color("#EA4335")
	colorMuted   = lipgloss.Color("#9AA0A6")
	colorBgLight = lipgloss.Color("#303134")
	colorAdded   = lipgloss.Color("#34A853")
)

// --- Styles -----------------------------------------------------------------

type styles struct {
	Container    lipgloss.Style
	Title        lipgloss.Style
	Header       lipgloss.Style
	Row          lipgloss.Style
	RowSelected  lipgloss.Style
	KeyCol       lipgloss.Style
	ValueCol     lipgloss.Style
	EffectCol    lipgloss.Style
	EffectPicker lipgloss.Style
	InputLabel   lipgloss.Style
	Help         lipgloss.Style
	Error        lipgloss.Style
	Divider      lipgloss.Style
	Added        lipgloss.Style
}

func defaultStyles() styles {
	return styles{
		Container: lipgloss.NewStyle().
			Padding(1, 2),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1),
		Header: lipgloss.NewStyle().
			Foreground(colorMuted).
			Bold(true),
		Row: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")),
		RowSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorBgLight),
		KeyCol: lipgloss.NewStyle().
			Foreground(colorPrimary),
		ValueCol: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")),
		EffectCol: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBC05")),
		EffectPicker: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Background(colorBgLight),
		InputLabel: lipgloss.NewStyle().
			Foreground(colorMuted),
		Help: lipgloss.NewStyle().
			Foreground(colorMuted),
		Error: lipgloss.NewStyle().
			Foreground(colorError),
		Divider: lipgloss.NewStyle().
			Foreground(colorMuted),
		Added: lipgloss.NewStyle().
			Foreground(colorAdded),
	}
}

// --- Key map ----------------------------------------------------------------

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Add     key.Binding
	Delete  key.Binding
	Edit    key.Binding
	Confirm key.Binding
	Cancel  key.Binding
	Save    key.Binding
	Tab     key.Binding
	Cycle   key.Binding // Space — cycle effect in edit mode
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add"),
		),
		Delete: key.NewBinding(
			key.WithKeys("x", "delete"),
			key.WithHelp("x/del", "delete"),
		),
		Edit: key.NewBinding(
			key.WithKeys("enter", "e"),
			key.WithHelp("enter/e", "edit"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "save"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next field"),
		),
		Cycle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "cycle effect"),
		),
	}
}

// --- Focus field constants --------------------------------------------------

type focusField int

const (
	focusKey focusField = iota
	focusValue
	focusEffect
)

// --- Messages ---------------------------------------------------------------

// SaveRequestedMsg is emitted when the user presses Ctrl+S.
type SaveRequestedMsg struct{}

// CancelRequestedMsg is emitted when the user presses Esc in navigation mode.
type CancelRequestedMsg struct{}

// --- Editor -----------------------------------------------------------------

// Editor is the taint editor component.
type Editor struct {
	// Taint data
	taints   []gcp.NodeTaint
	original []gcp.NodeTaint

	// UI state
	cursor      int
	editing     bool  // editing or adding mode
	adding      bool  // specifically adding a new taint
	editFocus   focusField
	editEffect  string // current effect value in the edit row
	keyInput    textinput.Model
	valueInput  textinput.Model
	err         string
	width       int
	height      int

	// Bindings and styles
	keys   keyMap
	styles styles
}

// New creates a new taint editor populated from initial.
func New(initial []gcp.NodeTaint) *Editor {
	taints := make([]gcp.NodeTaint, len(initial))
	copy(taints, initial)

	original := make([]gcp.NodeTaint, len(initial))
	copy(original, initial)

	keyInput := textinput.New()
	keyInput.Placeholder = "e.g. dedicated or example.com/gpu"
	keyInput.CharLimit = 253
	keyInput.Width = 35

	valueInput := textinput.New()
	valueInput.Placeholder = "value (optional)"
	valueInput.CharLimit = 63
	valueInput.Width = 25

	return &Editor{
		taints:     taints,
		original:   original,
		keyInput:   keyInput,
		valueInput: valueInput,
		editFocus:  focusKey,
		editEffect: EffectNoSchedule,
		keys:       defaultKeyMap(),
		styles:     defaultStyles(),
	}
}

// SetSize sets the editor dimensions and adjusts input widths.
func (e *Editor) SetSize(w, h int) {
	e.width = w
	e.height = h
	avail := w - 20
	if avail < 40 {
		avail = 40
	}
	keyW := avail * 2 / 5
	valW := avail * 2 / 5
	if keyW < 20 {
		keyW = 20
	}
	if valW < 15 {
		valW = 15
	}
	e.keyInput.Width = keyW
	e.valueInput.Width = valW
}

// GetTaints returns a copy of the current taint slice.
func (e *Editor) GetTaints() []gcp.NodeTaint {
	out := make([]gcp.NodeTaint, len(e.taints))
	copy(out, e.taints)
	return out
}

// IsDirty returns true when the taint list has changed from the initial snapshot.
func (e *Editor) IsDirty() bool {
	if len(e.taints) != len(e.original) {
		return true
	}
	a := append([]gcp.NodeTaint(nil), e.taints...)
	b := append([]gcp.NodeTaint(nil), e.original...)
	sort.Slice(a, func(i, j int) bool { return a[i].Key < a[j].Key })
	sort.Slice(b, func(i, j int) bool { return b[i].Key < b[j].Key })
	for i := range a {
		if a[i] != b[i] {
			return true
		}
	}
	return false
}

// IsEditing returns true when in edit/add mode.
func (e *Editor) IsEditing() bool {
	return e.editing || e.adding
}

// HasTextInputFocused returns true when a text input field has keyboard focus.
// The effect picker is NOT a text input, so it does not block global keys.
func (e *Editor) HasTextInputFocused() bool {
	return (e.editing || e.adding) && (e.editFocus == focusKey || e.editFocus == focusValue)
}

// Update processes a Bubble Tea message.
func (e *Editor) Update(msg tea.Msg) tea.Cmd {
	// Only clear error on key events (not on ticker/spinner messages).
	if _, isKey := msg.(tea.KeyMsg); isKey {
		e.err = ""
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	if e.editing || e.adding {
		return e.handleEditMode(keyMsg)
	}
	return e.handleNavMode(keyMsg)
}

// handleNavMode handles keys in navigation mode.
func (e *Editor) handleNavMode(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, e.keys.Up):
		if e.cursor > 0 {
			e.cursor--
		}
		return nil

	case key.Matches(msg, e.keys.Down):
		if e.cursor < len(e.taints)-1 {
			e.cursor++
		}
		return nil

	case key.Matches(msg, e.keys.Add):
		e.startAdding()
		return nil

	case key.Matches(msg, e.keys.Delete):
		e.deleteRow()
		return nil

	case key.Matches(msg, e.keys.Edit):
		if len(e.taints) > 0 {
			e.startEditing()
		}
		return nil

	case key.Matches(msg, e.keys.Save):
		return func() tea.Msg { return SaveRequestedMsg{} }

	case key.Matches(msg, e.keys.Cancel):
		return func() tea.Msg { return CancelRequestedMsg{} }
	}

	return nil
}

// handleEditMode handles keys in edit/add mode.
func (e *Editor) handleEditMode(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, e.keys.Cancel):
		e.cancelEdit()
		return nil

	case key.Matches(msg, e.keys.Tab):
		e.cycleEditFocus(+1)
		return nil

	case e.editFocus == focusEffect && key.Matches(msg, e.keys.Cycle):
		// Space cycles the effect picker.
		e.cycleEffect()
		return nil

	case e.editFocus == focusEffect && key.Matches(msg, e.keys.Confirm):
		// Enter on effect confirms and commits the row.
		return e.submitEdit()

	case e.editFocus != focusEffect && key.Matches(msg, e.keys.Confirm):
		// Enter on key/value advances focus to the next field.
		e.cycleEditFocus(+1)
		return nil

	default:
		// Delegate to the focused text input.
		return e.delegateToInput(msg)
	}
}

// cycleEditFocus advances focus to the next field (key → value → effect → key).
func (e *Editor) cycleEditFocus(delta int) {
	n := 3 // number of fields
	e.editFocus = focusField((int(e.editFocus) + delta + n) % n)
	e.updateInputFocus()
}

// cycleEffect advances the effect picker through the three values.
func (e *Editor) cycleEffect() {
	for i, ef := range allEffects {
		if ef == e.editEffect {
			e.editEffect = allEffects[(i+1)%len(allEffects)]
			return
		}
	}
	e.editEffect = EffectNoSchedule
}

// delegateToInput forwards a key message to the focused text input.
func (e *Editor) delegateToInput(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	switch e.editFocus {
	case focusKey:
		e.keyInput, cmd = e.keyInput.Update(msg)
	case focusValue:
		e.valueInput, cmd = e.valueInput.Update(msg)
	}
	return cmd
}

// startAdding begins adding a new taint.
func (e *Editor) startAdding() {
	e.adding = true
	e.editing = false
	e.editFocus = focusKey
	e.editEffect = EffectNoSchedule
	e.keyInput.SetValue("")
	e.valueInput.SetValue("")
	e.updateInputFocus()
}

// startEditing begins editing the selected taint.
func (e *Editor) startEditing() {
	if e.cursor < 0 || e.cursor >= len(e.taints) {
		return
	}
	t := e.taints[e.cursor]
	e.editing = true
	e.adding = false
	e.editFocus = focusKey
	e.editEffect = t.Effect
	if e.editEffect == "" {
		e.editEffect = EffectNoSchedule
	}
	e.keyInput.SetValue(t.Key)
	e.valueInput.SetValue(t.Value)
	e.updateInputFocus()
}

// cancelEdit reverts any in-progress edit and returns to navigation mode.
func (e *Editor) cancelEdit() {
	e.editing = false
	e.adding = false
	e.keyInput.Blur()
	e.valueInput.Blur()
}

// submitEdit validates and persists the edit row.
func (e *Editor) submitEdit() tea.Cmd {
	keyVal := strings.TrimSpace(e.keyInput.Value())
	valueVal := strings.TrimSpace(e.valueInput.Value())

	// Validate key.
	if keyVal == "" {
		e.err = "Key cannot be empty"
		return nil
	}
	if !taintKeyPattern.MatchString(keyVal) {
		e.err = "Key must match k8s naming rules (e.g. dedicated or example.com/gpu)"
		return nil
	}

	// Validate value.
	if valueVal != "" && !taintValuePattern.MatchString(valueVal) {
		e.err = "Value must be empty or match k8s naming rules"
		return nil
	}

	if e.adding {
		e.taints = append(e.taints, gcp.NodeTaint{
			Key:    keyVal,
			Value:  valueVal,
			Effect: e.editEffect,
		})
		e.cursor = len(e.taints) - 1
	} else {
		e.taints[e.cursor] = gcp.NodeTaint{
			Key:    keyVal,
			Value:  valueVal,
			Effect: e.editEffect,
		}
	}

	e.cancelEdit()
	return nil
}

// deleteRow removes the taint at the cursor position.
func (e *Editor) deleteRow() {
	if len(e.taints) == 0 {
		return
	}
	if e.cursor < 0 || e.cursor >= len(e.taints) {
		return
	}
	e.taints = append(e.taints[:e.cursor], e.taints[e.cursor+1:]...)
	if e.cursor >= len(e.taints) && e.cursor > 0 {
		e.cursor--
	}
}

// updateInputFocus synchronizes focus state on the text inputs.
func (e *Editor) updateInputFocus() {
	switch e.editFocus {
	case focusKey:
		e.keyInput.Focus()
		e.valueInput.Blur()
	case focusValue:
		e.keyInput.Blur()
		e.valueInput.Focus()
	default:
		e.keyInput.Blur()
		e.valueInput.Blur()
	}
}

// View renders the editor.
func (e *Editor) View() string {
	var b strings.Builder

	// Title.
	b.WriteString(e.styles.Title.Render("Edit Taints"))
	b.WriteString(e.styles.Help.Render(" (" + itoa(len(e.taints)) + ")"))
	b.WriteString("\n\n")

	// Inline edit form when adding or editing.
	if e.editing || e.adding {
		action := "Edit"
		if e.adding {
			action = "Add"
		}
		b.WriteString(e.styles.InputLabel.Render(action + " Taint"))
		b.WriteString("\n")

		// Key input row.
		keyLabel := "  Key:    "
		if e.editFocus == focusKey {
			keyLabel = "▶ Key:    "
		}
		b.WriteString(e.styles.InputLabel.Render(keyLabel))
		b.WriteString(e.keyInput.View())
		b.WriteString("\n")

		// Value input row.
		valLabel := "  Value:  "
		if e.editFocus == focusValue {
			valLabel = "▶ Value:  "
		}
		b.WriteString(e.styles.InputLabel.Render(valLabel))
		b.WriteString(e.valueInput.View())
		b.WriteString("\n")

		// Effect picker row.
		effLabel := "  Effect: "
		if e.editFocus == focusEffect {
			effLabel = "▶ Effect: "
		}
		b.WriteString(e.styles.InputLabel.Render(effLabel))
		if e.editFocus == focusEffect {
			b.WriteString(e.styles.EffectPicker.Render(" " + e.editEffect + " "))
			b.WriteString(e.styles.Help.Render("  (space to cycle)"))
		} else {
			b.WriteString(e.styles.EffectCol.Render(e.editEffect))
		}
		b.WriteString("\n")

		// Validation error.
		if e.err != "" {
			b.WriteString(e.styles.Error.Render("  " + e.err))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(e.styles.Divider.Render(strings.Repeat("─", 48)))
		b.WriteString("\n\n")
	}

	// Column header.
	b.WriteString(e.styles.Header.Render("  KEY"))
	b.WriteString(e.styles.Header.Render(strings.Repeat(" ", 28) + "VALUE"))
	b.WriteString(e.styles.Header.Render(strings.Repeat(" ", 14) + "EFFECT"))
	b.WriteString("\n")

	// Taint rows.
	if len(e.taints) == 0 {
		b.WriteString(e.styles.Help.Render("  No taints. Press 'a' to add one."))
		b.WriteString("\n")
	} else {
		for i, t := range e.taints {
			b.WriteString(e.renderRow(t, i == e.cursor))
			b.WriteString("\n")
		}
	}

	// Help text.
	b.WriteString("\n")
	if e.editing || e.adding {
		b.WriteString(e.styles.Help.Render("tab:next field  space:cycle effect  enter:confirm  esc:cancel"))
	} else {
		b.WriteString(e.styles.Help.Render("↑↓:move  a:add  e/enter:edit  x/del:delete  ctrl+s:save  esc:cancel"))
	}

	return e.styles.Container.Render(b.String())
}

// renderRow renders a single taint as a fixed-width row.
func (e *Editor) renderRow(t gcp.NodeTaint, selected bool) string {
	cursor := "  "
	if selected {
		cursor = symbols.Cursor() + " "
	}

	keyStyle := e.styles.KeyCol
	valStyle := e.styles.ValueCol
	effStyle := e.styles.EffectCol
	if selected {
		keyStyle = e.styles.KeyCol.Background(colorBgLight).Bold(true)
		valStyle = e.styles.ValueCol.Background(colorBgLight).Bold(true)
		effStyle = e.styles.EffectCol.Background(colorBgLight).Bold(true)
	}

	key := padRight(t.Key, 30)
	val := padRight(t.Value, 18)
	eff := t.Effect

	return cursor + keyStyle.Render(key) + "  " + valStyle.Render(val) + "  " + effStyle.Render(eff)
}

// padRight pads s to at least n visible characters (truncates if longer).
func padRight(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return string(r[:n])
	}
	return s + strings.Repeat(" ", n-len(r))
}

// itoa converts a non-negative int to a decimal string.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
