// Package labeledit provides a component for editing key-value label pairs
// used in GCP resources like VM instances.
package labeledit

import (
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/symbols"
)

// GCP label validation: lowercase letters, numbers, hyphens, underscores
// Keys must start with a lowercase letter
var (
	keyPattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	valuePattern = regexp.MustCompile(`^[a-z0-9_-]*$`)
)

// Colors
var (
	colorPrimary   = lipgloss.Color("#4285F4")
	colorSecondary = lipgloss.Color("#34A853")
	colorError     = lipgloss.Color("#EA4335")
	colorMuted     = lipgloss.Color("#9AA0A6")
	colorBgLight   = lipgloss.Color("#303134")
)

// keyMap defines key bindings for the label editor
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
	}
}

type styles struct {
	Container       lipgloss.Style
	Title           lipgloss.Style
	Label           lipgloss.Style
	LabelSelected   lipgloss.Style
	Key             lipgloss.Style
	Value           lipgloss.Style
	KeySelected     lipgloss.Style
	ValueSelected   lipgloss.Style
	InputLabel      lipgloss.Style
	Help            lipgloss.Style
	Error           lipgloss.Style
	Divider         lipgloss.Style
	Added           lipgloss.Style
	Modified        lipgloss.Style
	MarkedForDelete lipgloss.Style
}

func defaultStyles() styles {
	return styles{
		Container: lipgloss.NewStyle().
			Padding(1, 2),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1),
		Label: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")),
		LabelSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorBgLight),
		Key: lipgloss.NewStyle().
			Foreground(colorPrimary),
		Value: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")),
		KeySelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Background(colorBgLight),
		ValueSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
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
			Foreground(colorSecondary),
		Modified: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBC05")),
		MarkedForDelete: lipgloss.NewStyle().
			Foreground(colorError).
			Strikethrough(true),
	}
}

// labelEntry represents a single label with its state
type labelEntry struct {
	key             string
	value           string
	originalKey     string
	originalValue   string
	isNew           bool
	markedForDelete bool
}

// Editor is the label editor component
type Editor struct {
	// Label data
	entries        []labelEntry
	originalLabels map[string]string

	// UI state
	cursor     int
	editing    bool
	adding     bool
	focusKey   bool // When editing: true=editing key, false=editing value
	keyInput   textinput.Model
	valueInput textinput.Model
	err        string
	width      int
	height     int

	// Bindings and styles
	keys   keyMap
	styles styles
}

// New creates a new label editor with the given labels
func New(labels map[string]string) *Editor {
	// Create sorted entries from labels
	var entries []labelEntry
	if labels != nil {
		keys := make([]string, 0, len(labels))
		for k := range labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			entries = append(entries, labelEntry{
				key:           k,
				value:         labels[k],
				originalKey:   k,
				originalValue: labels[k],
				isNew:         false,
			})
		}
	}

	// Store original labels for dirty checking
	originalLabels := make(map[string]string)
	for k, v := range labels {
		originalLabels[k] = v
	}

	// Create text inputs
	keyInput := textinput.New()
	keyInput.Placeholder = "key"
	keyInput.CharLimit = 63 // GCP label key limit
	keyInput.Width = 30

	valueInput := textinput.New()
	valueInput.Placeholder = "value"
	valueInput.CharLimit = 63 // GCP label value limit
	valueInput.Width = 30

	return &Editor{
		entries:        entries,
		originalLabels: originalLabels,
		keyInput:       keyInput,
		valueInput:     valueInput,
		focusKey:       true,
		keys:           defaultKeyMap(),
		styles:         defaultStyles(),
	}
}

// SetSize sets the editor dimensions
func (e *Editor) SetSize(width, height int) {
	e.width = width
	e.height = height
	inputWidth := (width - 20) / 2
	if inputWidth < 20 {
		inputWidth = 20
	}
	e.keyInput.Width = inputWidth
	e.valueInput.Width = inputWidth
}

// GetLabels returns the current labels (excluding those marked for deletion)
func (e *Editor) GetLabels() map[string]string {
	result := make(map[string]string)
	for _, entry := range e.entries {
		if !entry.markedForDelete && entry.key != "" {
			result[entry.key] = entry.value
		}
	}
	return result
}

// IsDirty returns true if labels have been modified
func (e *Editor) IsDirty() bool {
	current := e.GetLabels()

	// Check if any original labels are missing or changed
	for k, v := range e.originalLabels {
		if cv, exists := current[k]; !exists || cv != v {
			return true
		}
	}

	// Check if any new labels were added
	for k := range current {
		if _, exists := e.originalLabels[k]; !exists {
			return true
		}
	}

	return false
}

// IsEditing returns true if currently in edit mode
func (e *Editor) IsEditing() bool {
	return e.editing || e.adding
}

// Update handles input messages
func (e *Editor) Update(msg tea.Msg) tea.Cmd {
	// Clear error on any input
	e.err = ""

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	// Handle editing/adding mode
	if e.editing || e.adding {
		return e.handleEditMode(keyMsg)
	}

	// Normal navigation mode
	return e.handleNavigationMode(keyMsg)
}

// handleNavigationMode handles keys when not editing
func (e *Editor) handleNavigationMode(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, e.keys.Up):
		if e.cursor > 0 {
			e.cursor--
		}
		return nil

	case key.Matches(msg, e.keys.Down):
		if e.cursor < len(e.entries)-1 {
			e.cursor++
		}
		return nil

	case key.Matches(msg, e.keys.Add):
		e.startAdding()
		return nil

	case key.Matches(msg, e.keys.Delete):
		e.toggleDelete()
		return nil

	case key.Matches(msg, e.keys.Edit):
		if len(e.entries) > 0 {
			e.startEditing()
		}
		return nil

	case key.Matches(msg, e.keys.Save):
		return func() tea.Msg { return SaveRequestedMsg{} }
	}

	return nil
}

// handleEditMode handles keys when editing or adding
func (e *Editor) handleEditMode(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, e.keys.Cancel):
		e.cancelEdit()
		return nil

	case key.Matches(msg, e.keys.Tab):
		// Toggle between key and value inputs
		e.focusKey = !e.focusKey
		e.updateInputFocus()
		return nil

	case key.Matches(msg, e.keys.Confirm):
		if e.focusKey {
			// Move to value input
			e.focusKey = false
			e.updateInputFocus()
			return nil
		}
		// Submit the edit
		return e.submitEdit()

	default:
		// Update the focused input
		var cmd tea.Cmd
		if e.focusKey {
			e.keyInput, cmd = e.keyInput.Update(msg)
		} else {
			e.valueInput, cmd = e.valueInput.Update(msg)
		}
		return cmd
	}
}

// startAdding begins adding a new label
func (e *Editor) startAdding() {
	e.adding = true
	e.editing = false
	e.focusKey = true
	e.keyInput.SetValue("")
	e.valueInput.SetValue("")
	e.updateInputFocus()
}

// startEditing begins editing the selected label
func (e *Editor) startEditing() {
	if e.cursor < 0 || e.cursor >= len(e.entries) {
		return
	}

	entry := e.entries[e.cursor]
	if entry.markedForDelete {
		return // Can't edit deleted entries
	}

	e.editing = true
	e.adding = false
	e.focusKey = false // Start with value focused since key changes less often
	e.keyInput.SetValue(entry.key)
	e.valueInput.SetValue(entry.value)
	e.updateInputFocus()
}

// cancelEdit cancels the current edit operation
func (e *Editor) cancelEdit() {
	e.editing = false
	e.adding = false
	e.keyInput.Blur()
	e.valueInput.Blur()
}

// submitEdit validates and applies the current edit
func (e *Editor) submitEdit() tea.Cmd {
	keyVal := strings.TrimSpace(e.keyInput.Value())
	valueVal := strings.TrimSpace(e.valueInput.Value())

	// Validate key
	if keyVal == "" {
		e.err = "Key cannot be empty"
		return nil
	}
	if !keyPattern.MatchString(keyVal) {
		e.err = "Key must start with lowercase letter, contain only lowercase letters, numbers, hyphens, underscores"
		return nil
	}

	// Validate value (can be empty)
	if valueVal != "" && !valuePattern.MatchString(valueVal) {
		e.err = "Value must contain only lowercase letters, numbers, hyphens, underscores"
		return nil
	}

	// Check for duplicate keys (excluding current entry if editing)
	for i, entry := range e.entries {
		if entry.key == keyVal && !entry.markedForDelete {
			if e.adding || (e.editing && i != e.cursor) {
				e.err = "Key already exists"
				return nil
			}
		}
	}

	if e.adding {
		// Add new entry
		e.entries = append(e.entries, labelEntry{
			key:   keyVal,
			value: valueVal,
			isNew: true,
		})
		e.cursor = len(e.entries) - 1
	} else {
		// Update existing entry
		e.entries[e.cursor].key = keyVal
		e.entries[e.cursor].value = valueVal
	}

	e.cancelEdit()
	return nil
}

// toggleDelete marks/unmarks the current entry for deletion
func (e *Editor) toggleDelete() {
	if e.cursor < 0 || e.cursor >= len(e.entries) {
		return
	}

	entry := &e.entries[e.cursor]

	if entry.isNew {
		// Remove new entries immediately
		e.entries = append(e.entries[:e.cursor], e.entries[e.cursor+1:]...)
		if e.cursor >= len(e.entries) && e.cursor > 0 {
			e.cursor--
		}
	} else {
		// Toggle delete mark for existing entries
		entry.markedForDelete = !entry.markedForDelete
	}
}

// updateInputFocus updates which input has focus
func (e *Editor) updateInputFocus() {
	if e.focusKey {
		e.keyInput.Focus()
		e.valueInput.Blur()
	} else {
		e.keyInput.Blur()
		e.valueInput.Focus()
	}
}

// View renders the label editor
func (e *Editor) View() string {
	var b strings.Builder

	// Title with count
	count := 0
	for _, entry := range e.entries {
		if !entry.markedForDelete {
			count++
		}
	}
	b.WriteString(e.styles.Title.Render("Labels"))
	b.WriteString(e.styles.Help.Render(" (" + itoa(count) + ")"))
	b.WriteString("\n\n")

	// Edit/Add input area
	if e.editing || e.adding {
		action := "Edit"
		if e.adding {
			action = "Add"
		}
		b.WriteString(e.styles.InputLabel.Render(action + " Label"))
		b.WriteString("\n")

		// Key input
		keyLabel := "  Key: "
		if e.focusKey {
			keyLabel = "▶ Key: "
		}
		b.WriteString(e.styles.InputLabel.Render(keyLabel))
		b.WriteString(e.keyInput.View())
		b.WriteString("\n")

		// Value input
		valueLabel := "  Value: "
		if !e.focusKey {
			valueLabel = "▶ Value: "
		}
		b.WriteString(e.styles.InputLabel.Render(valueLabel))
		b.WriteString(e.valueInput.View())
		b.WriteString("\n")

		// Error message
		if e.err != "" {
			b.WriteString(e.styles.Error.Render("  " + e.err))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(e.styles.Divider.Render(strings.Repeat("─", 40)))
		b.WriteString("\n\n")
	}

	// Label list
	if len(e.entries) == 0 {
		b.WriteString(e.styles.Help.Render("  No labels. Press 'a' to add one."))
		b.WriteString("\n")
	} else {
		for i, entry := range e.entries {
			line := e.renderEntry(entry, i == e.cursor)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Help text
	b.WriteString("\n")
	if e.editing || e.adding {
		b.WriteString(e.styles.Help.Render("tab:switch field  enter:confirm  esc:cancel"))
	} else {
		b.WriteString(e.styles.Help.Render("j/k:nav  a:add  e/enter:edit  x:delete  ctrl+s:save"))
	}

	return e.styles.Container.Render(b.String())
}

// renderEntry renders a single label entry
func (e *Editor) renderEntry(entry labelEntry, selected bool) string {
	cursor := "  "
	if selected {
		cursor = symbols.Cursor() + " "
	}

	// Status indicator
	var status string
	switch {
	case entry.markedForDelete:
		status = e.styles.MarkedForDelete.Render("×")
	case entry.isNew:
		status = e.styles.Added.Render("+")
	case entry.key != entry.originalKey || entry.value != entry.originalValue:
		status = e.styles.Modified.Render("~")
	default:
		status = " "
	}

	// Key and value styles based on entry state
	var keyStyle, valueStyle lipgloss.Style
	switch {
	case entry.markedForDelete:
		keyStyle = e.styles.MarkedForDelete
		valueStyle = e.styles.MarkedForDelete
	case selected:
		keyStyle = e.styles.KeySelected
		valueStyle = e.styles.ValueSelected
	default:
		keyStyle = e.styles.Key
		valueStyle = e.styles.Value
	}

	keyText := keyStyle.Render(entry.key)
	valueText := valueStyle.Render(entry.value)

	return cursor + status + " " + keyText + " = " + valueText
}

// itoa converts int to string (simple helper)
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

// SaveRequestedMsg is emitted when user presses Ctrl+S
type SaveRequestedMsg struct{}

// HasTextInputFocused returns true when a text input is focused.
// Used to prevent global keys (like 'q' for quit) from being triggered while typing.
func (e *Editor) HasTextInputFocused() bool {
	return e.editing || e.adding
}
