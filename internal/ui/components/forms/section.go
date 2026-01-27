package forms

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Section styling colors
var (
	sectionColorPrimary = lipgloss.Color("#4285F4")
	sectionColorMuted   = lipgloss.Color("#9AA0A6")
)

// sectionKeyMap defines keys for section navigation
type sectionKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Tab      key.Binding
	Toggle   key.Binding
	Collapse key.Binding
}

func defaultSectionKeyMap() sectionKeyMap {
	return sectionKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "previous field"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "next field"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next field"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "toggle section"),
		),
		Collapse: key.NewBinding(
			key.WithKeys("-"),
			key.WithHelp("-", "collapse section"),
		),
	}
}

// sectionStyles defines styles for form sections
type sectionStyles struct {
	Title          lipgloss.Style
	TitleCollapsed lipgloss.Style
	Container      lipgloss.Style
	Divider        lipgloss.Style
}

func defaultSectionStyles() sectionStyles {
	return sectionStyles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(sectionColorPrimary).
			MarginBottom(1),
		TitleCollapsed: lipgloss.NewStyle().
			Foreground(sectionColorMuted),
		Container: lipgloss.NewStyle().
			MarginBottom(1).
			PaddingLeft(2),
		Divider: lipgloss.NewStyle().
			Foreground(sectionColorMuted),
	}
}

// Section represents a group of related form fields
type Section struct {
	// Configuration
	ID          string
	Title       string
	Icon        string
	Description string
	Collapsible bool
	Collapsed   bool

	// Fields
	fields     []*Field
	focusIndex int

	// UI state
	focused bool
	width   int
	height  int

	// Styling
	styles sectionStyles
	keys   sectionKeyMap
}

// NewSection creates a new form section
func NewSection(id, title string) *Section {
	return &Section{
		ID:         id,
		Title:      title,
		focusIndex: -1,
		styles:     defaultSectionStyles(),
		keys:       defaultSectionKeyMap(),
	}
}

// Builder methods

// SetIcon sets the section icon/prefix
func (s *Section) SetIcon(icon string) *Section {
	s.Icon = icon
	return s
}

// SetDescription sets the section description
func (s *Section) SetDescription(desc string) *Section {
	s.Description = desc
	return s
}

// SetCollapsible makes the section collapsible
func (s *Section) SetCollapsible(collapsible bool) *Section {
	s.Collapsible = collapsible
	return s
}

// SetCollapsed sets the initial collapsed state
func (s *Section) SetCollapsed(collapsed bool) *Section {
	s.Collapsed = collapsed
	return s
}

// AddField adds a field to the section
func (s *Section) AddField(field *Field) *Section {
	s.fields = append(s.fields, field)
	return s
}

// Fields returns all fields in the section
func (s *Section) Fields() []*Field {
	return s.fields
}

// FieldCount returns the number of fields
func (s *Section) FieldCount() int {
	return len(s.fields)
}

// GetField returns a field by ID
func (s *Section) GetField(id string) *Field {
	for _, f := range s.fields {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// EditableFieldCount returns the number of editable fields
func (s *Section) EditableFieldCount() int {
	count := 0
	for _, f := range s.fields {
		if f.IsEditable() {
			count++
		}
	}
	return count
}

// Focus management

// Focus sets focus on the section
func (s *Section) Focus() {
	s.focused = true
	// Don't focus fields if section is collapsed
	if s.Collapsed {
		return
	}
	// Focus first editable field if none focused
	if s.focusIndex < 0 {
		s.FocusFirstEditable()
	} else if s.focusIndex < len(s.fields) {
		s.fields[s.focusIndex].Focus()
	}
}

// Blur removes focus from the section
func (s *Section) Blur() {
	s.focused = false
	if s.focusIndex >= 0 && s.focusIndex < len(s.fields) {
		s.fields[s.focusIndex].Blur()
	}
}

// IsFocused returns true if the section is focused
func (s *Section) IsFocused() bool {
	return s.focused
}

// FocusedField returns the currently focused field
func (s *Section) FocusedField() *Field {
	if s.focusIndex >= 0 && s.focusIndex < len(s.fields) {
		return s.fields[s.focusIndex]
	}
	return nil
}

// HasTextInputFocused returns true if a text input field is currently focused
func (s *Section) HasTextInputFocused() bool {
	if field := s.FocusedField(); field != nil {
		return field.IsFocused() && field.IsTextInput()
	}
	return false
}

// FocusedFieldIndex returns the index of the focused field
func (s *Section) FocusedFieldIndex() int {
	return s.focusIndex
}

// FocusFirstEditable focuses the first editable field
func (s *Section) FocusFirstEditable() bool {
	for i, f := range s.fields {
		if f.IsEditable() {
			s.setFocusIndex(i)
			return true
		}
	}
	return false
}

// FocusLastEditable focuses the last editable field
func (s *Section) FocusLastEditable() bool {
	for i := len(s.fields) - 1; i >= 0; i-- {
		if s.fields[i].IsEditable() {
			s.setFocusIndex(i)
			return true
		}
	}
	return false
}

// NextField moves focus to the next editable field
// Returns true if focus moved within section, false if at end
func (s *Section) NextField() bool {
	// Find next editable field after current
	for i := s.focusIndex + 1; i < len(s.fields); i++ {
		if s.fields[i].IsEditable() {
			s.setFocusIndex(i)
			return true
		}
	}
	return false
}

// PrevField moves focus to the previous editable field
// Returns true if focus moved within section, false if at start
func (s *Section) PrevField() bool {
	// Find previous editable field before current
	for i := s.focusIndex - 1; i >= 0; i-- {
		if s.fields[i].IsEditable() {
			s.setFocusIndex(i)
			return true
		}
	}
	return false
}

// setFocusIndex changes focus to a specific field index
func (s *Section) setFocusIndex(index int) {
	// Blur current field
	if s.focusIndex >= 0 && s.focusIndex < len(s.fields) {
		s.fields[s.focusIndex].Blur()
	}

	// Focus new field
	s.focusIndex = index
	if index >= 0 && index < len(s.fields) {
		s.fields[index].Focus()
	}
}

// SetSize sets the section dimensions
func (s *Section) SetSize(width, height int) {
	s.width = width
	s.height = height

	// Propagate to fields
	for _, f := range s.fields {
		f.SetSize(width-4, 0) // Leave room for padding
	}
}

// Validation

// Validate validates all fields in the section
func (s *Section) Validate() []error {
	var errors []error
	for _, f := range s.fields {
		if err := f.Validate(); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// HasErrors returns true if any field has validation errors
func (s *Section) HasErrors() bool {
	for _, f := range s.fields {
		if f.HasError() {
			return true
		}
	}
	return false
}

// IsDirty returns true if any field has changed
func (s *Section) IsDirty() bool {
	for _, f := range s.fields {
		if f.IsDirty() {
			return true
		}
	}
	return false
}

// GetData returns all field values as a map
func (s *Section) GetData() map[string]interface{} {
	data := make(map[string]interface{})
	for _, f := range s.fields {
		data[f.ID] = f.GetValue()
	}
	return data
}

// ToggleCollapse toggles the collapsed state
func (s *Section) ToggleCollapse() {
	if s.Collapsible {
		s.Collapsed = !s.Collapsed
	}
}

// Update handles input messages
//nolint:gocognit // Section input handling - complexity 38
func (s *Section) Update(msg tea.Msg) tea.Cmd {
	if s.Collapsed {
		// Only handle expand when collapsed
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if key.Matches(keyMsg, s.keys.Toggle) {
				s.ToggleCollapse()
				// Focus first editable field after expanding
				if !s.Collapsed {
					s.FocusFirstEditable()
				}
				return nil
			}
		}
		return nil
	}

	// Check for field navigation keys first
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, s.keys.Collapse):
			// Collapse the section with "-" key
			if s.Collapsible {
				s.Collapsed = true
				return nil
			}
			return nil

		case key.Matches(keyMsg, s.keys.Down), key.Matches(keyMsg, s.keys.Tab):
			// Only navigate if the current field isn't capturing input
			if field := s.FocusedField(); field != nil {
				// For dropdowns with open menu, let them handle the key
				if field.dropdownOpen {
					return field.Update(msg)
				}
			}
			if !s.NextField() {
				// At end of section, return nil to let parent handle
				return nil
			}
			return nil

		case key.Matches(keyMsg, s.keys.Up):
			if field := s.FocusedField(); field != nil {
				if field.dropdownOpen {
					return field.Update(msg)
				}
			}
			if !s.PrevField() {
				// At start of section
				return nil
			}
			return nil
		}
	}

	// Delegate to focused field
	if s.focusIndex >= 0 && s.focusIndex < len(s.fields) {
		return s.fields[s.focusIndex].Update(msg)
	}

	return nil
}

// View renders the section
func (s *Section) View() string {
	var b strings.Builder

	// Section title with collapse indicator (rendered outside container to avoid clipping issues)
	titleText := s.Title
	if s.Icon != "" {
		titleText = s.Icon + " " + titleText
	}

	if s.Collapsible {
		indicator := "▾"
		if s.Collapsed {
			indicator = "▸"
		}
		titleText = indicator + " " + titleText

		// Show hint when section is focused
		if s.focused {
			if s.Collapsed {
				titleText += "  (press Enter to expand)"
			} else {
				titleText += "  (press - to collapse)"
			}
		}
	}

	titleStyle := s.styles.Title
	if s.Collapsed {
		titleStyle = s.styles.TitleCollapsed
		// Highlight collapsed section when focused
		if s.focused {
			titleStyle = s.styles.Title // Use primary color when focused
		}
	}
	b.WriteString(titleStyle.Render(titleText))
	b.WriteString("\n")

	// Content inside container (fields with padding)
	var content strings.Builder

	// Description if provided
	if s.Description != "" && !s.Collapsed {
		content.WriteString(s.styles.Divider.Render(s.Description))
		content.WriteString("\n")
	}

	// Fields (only if not collapsed)
	if !s.Collapsed {
		for _, field := range s.fields {
			content.WriteString(field.View())
			content.WriteString("\n")
		}
	}

	// Only wrap content in container if there's content to show
	if content.Len() > 0 {
		b.WriteString(s.styles.Container.Render(content.String()))
	}
	b.WriteString("\n") // Add margin between sections

	return b.String()
}
