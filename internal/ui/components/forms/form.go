package forms

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Form colors
var (
	formColorPrimary = lipgloss.Color("#4285F4")
	formColorError   = lipgloss.Color("#EA4335")
	formColorMuted   = lipgloss.Color("#9AA0A6")
	formColorWhite   = lipgloss.Color("#FFFFFF")
)

// formKeyMap defines key bindings for the form
type formKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	Submit   key.Binding
	Cancel   key.Binding
	Help     key.Binding
	Select   key.Binding
}

func defaultFormKeyMap() formKeyMap {
	return formKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "previous"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "next"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next field"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "previous field"),
		),
		Submit: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "submit"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
	}
}

// formStyles defines styles for the form
type formStyles struct {
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Container   lipgloss.Style
	Help        lipgloss.Style
	Error       lipgloss.Style
	ErrorBox    lipgloss.Style
	Success     lipgloss.Style
	ActionBar   lipgloss.Style
	Button      lipgloss.Style
	ButtonFocus lipgloss.Style
}

func defaultFormStyles() formStyles {
	return formStyles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(formColorPrimary).
			MarginBottom(1),
		Subtitle: lipgloss.NewStyle().
			Foreground(formColorMuted),
		Container: lipgloss.NewStyle().
			Padding(1, 2),
		Help: lipgloss.NewStyle().
			Foreground(formColorMuted),
		Error: lipgloss.NewStyle().
			Foreground(formColorError),
		ErrorBox: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(formColorError).
			Padding(0, 1).
			MarginBottom(1),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#34A853")),
		ActionBar: lipgloss.NewStyle().
			MarginTop(1).
			Padding(1, 0).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(formColorMuted).
			BorderTop(true),
		Button: lipgloss.NewStyle().
			Foreground(formColorMuted).
			Padding(0, 2),
		ButtonFocus: lipgloss.NewStyle().
			Bold(true).
			Foreground(formColorWhite).
			Background(formColorPrimary).
			Padding(0, 2),
	}
}

// Form represents a complete form with multiple sections
type Form struct {
	// Configuration
	Title    string
	Subtitle string
	Mode     FormMode

	// Sections
	sections         []*Section
	focusSectionIdx  int
	focusedOnActions bool
	actionIndex      int // 0=Submit, 1=Cancel

	// UI state
	showHelp     bool
	showErrors   bool
	errors       []string
	viewport     viewport.Model
	useViewport  bool
	width        int
	height       int
	contentReady bool

	// Styling
	styles formStyles
	keys   formKeyMap
}

// NewForm creates a new form
func NewForm(title string, mode FormMode) *Form {
	return &Form{
		Title:           title,
		Mode:            mode,
		focusSectionIdx: 0,
		styles:          defaultFormStyles(),
		keys:            defaultFormKeyMap(),
	}
}

// Builder methods

// SetSubtitle sets the form subtitle
func (f *Form) SetSubtitle(subtitle string) *Form {
	f.Subtitle = subtitle
	return f
}

// AddSection adds a section to the form
func (f *Form) AddSection(section *Section) *Form {
	f.sections = append(f.sections, section)
	return f
}

// Sections returns all sections
func (f *Form) Sections() []*Section {
	return f.sections
}

// SectionCount returns the number of sections
func (f *Form) SectionCount() int {
	return len(f.sections)
}

// GetSection returns a section by ID
func (f *Form) GetSection(id string) *Section {
	for _, s := range f.sections {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// GetField returns a field by ID from any section
func (f *Form) GetField(id string) *Field {
	for _, s := range f.sections {
		if field := s.GetField(id); field != nil {
			return field
		}
	}
	return nil
}

// Focus management

// FocusedSection returns the currently focused section
func (f *Form) FocusedSection() *Section {
	if f.focusSectionIdx >= 0 && f.focusSectionIdx < len(f.sections) {
		return f.sections[f.focusSectionIdx]
	}
	return nil
}

// HasTextInputFocused returns true if a text input field is currently focused.
// This is used by the app to know whether to consume character keys or pass them
// to global handlers (e.g., "q" for quit).
func (f *Form) HasTextInputFocused() bool {
	if f.focusedOnActions {
		return false
	}
	if section := f.FocusedSection(); section != nil {
		return section.HasTextInputFocused()
	}
	return false
}

// Focus sets initial focus on the form
func (f *Form) Focus() {
	if len(f.sections) > 0 {
		f.sections[0].Focus()
	}
}

// Init initializes the form
func (f *Form) Init() tea.Cmd {
	// Focus first section with editable fields
	for i, s := range f.sections {
		if s.EditableFieldCount() > 0 {
			f.focusSectionIdx = i
			s.Focus()
			break
		}
	}
	return nil
}

// nextField moves to the next field across sections
func (f *Form) nextField() {
	if f.focusedOnActions {
		return // Can't go forward from actions
	}

	currentSection := f.FocusedSection()
	if currentSection == nil {
		return
	}

	// Try to move within current section (only if not collapsed)
	if !currentSection.Collapsed && currentSection.NextField() {
		return
	}

	// Move to next section
	for i := f.focusSectionIdx + 1; i < len(f.sections); i++ {
		section := f.sections[i]
		// Allow focus on: non-collapsed sections with fields, OR collapsible sections (even if collapsed)
		if (section.EditableFieldCount() > 0 && !section.Collapsed) || section.Collapsible {
			currentSection.Blur()
			f.focusSectionIdx = i
			section.Focus()
			if !section.Collapsed {
				section.FocusFirstEditable()
			}
			return
		}
	}

	// At end of form - focus actions
	currentSection.Blur()
	f.focusedOnActions = true
}

// prevField moves to the previous field across sections
func (f *Form) prevField() {
	if f.focusedOnActions {
		// Move back from actions to last field
		f.focusedOnActions = false
		for i := len(f.sections) - 1; i >= 0; i-- {
			section := f.sections[i]
			// Allow focus on: non-collapsed sections with fields, OR collapsible sections
			if (section.EditableFieldCount() > 0 && !section.Collapsed) || section.Collapsible {
				f.focusSectionIdx = i
				section.Focus()
				if !section.Collapsed {
					section.FocusLastEditable()
				}
				return
			}
		}
		return
	}

	currentSection := f.FocusedSection()
	if currentSection == nil {
		return
	}

	// Try to move within current section (only if not collapsed)
	if !currentSection.Collapsed && currentSection.PrevField() {
		return
	}

	// Move to previous section
	for i := f.focusSectionIdx - 1; i >= 0; i-- {
		section := f.sections[i]
		// Allow focus on: non-collapsed sections with fields, OR collapsible sections
		if (section.EditableFieldCount() > 0 && !section.Collapsed) || section.Collapsible {
			currentSection.Blur()
			f.focusSectionIdx = i
			section.Focus()
			if !section.Collapsed {
				section.FocusLastEditable()
			}
			return
		}
	}
}

// SetSize sets the form dimensions
func (f *Form) SetSize(width, height int) {
	f.width = width
	f.height = height

	// Calculate content height for viewport
	contentHeight := height - 8 // Leave room for title, subtitle, actions, help
	if contentHeight < 10 {
		contentHeight = 10
	}

	// Initialize viewport if needed
	if !f.contentReady {
		f.viewport = viewport.New(width-4, contentHeight)
		f.viewport.Style = lipgloss.NewStyle()
		f.contentReady = true
	} else {
		f.viewport.Width = width - 4
		f.viewport.Height = contentHeight
	}

	// Propagate to sections
	for _, s := range f.sections {
		s.SetSize(width-4, 0)
	}
}

// EnableViewport enables viewport scrolling for long forms
func (f *Form) EnableViewport() *Form {
	f.useViewport = true
	return f
}

// scrollToFocused scrolls the viewport to keep the focused field visible
func (f *Form) scrollToFocused() {
	if !f.useViewport || !f.contentReady {
		return
	}

	// When action bar is focused, scroll to the bottom of content
	if f.focusedOnActions {
		// Calculate total content height and scroll to bottom
		totalLines := 0
		for _, section := range f.sections {
			if section.Collapsed {
				totalLines += 2
			} else {
				headerLines := 2
				if section.Description != "" {
					headerLines++
				}
				totalLines += headerLines + len(section.Fields())*4
			}
		}
		// Scroll to show the bottom of content (action bar is rendered below viewport)
		maxOffset := totalLines - f.viewport.Height
		if maxOffset > 0 {
			f.viewport.SetYOffset(maxOffset)
		}
		return
	}

	// Calculate the line position of the focused field
	linePos := 0
	for i, section := range f.sections {
		// Section header takes ~2-3 lines
		sectionHeaderLines := 2
		if section.Description != "" {
			sectionHeaderLines++
		}

		if i == f.focusSectionIdx {
			// Found the focused section
			linePos += sectionHeaderLines

			// Add lines for fields before the focused one
			focusedFieldIdx := section.FocusedFieldIndex()
			for j := 0; j < focusedFieldIdx && j < len(section.Fields()); j++ {
				// Each field takes ~3-4 lines (label, input, help/error, margin)
				linePos += 4
			}
			break
		}

		// Add all lines for this section
		if section.Collapsed {
			linePos += 2 // Just the header
		} else {
			linePos += sectionHeaderLines + len(section.Fields())*4
		}
	}

	// Scroll viewport to show the focused field with some padding
	viewportHeight := f.viewport.Height
	currentTop := f.viewport.YOffset

	// If field is above viewport, scroll up
	if linePos < currentTop+2 {
		f.viewport.SetYOffset(max(0, linePos-2))
	}
	// If field is below viewport, scroll down
	if linePos > currentTop+viewportHeight-4 {
		f.viewport.SetYOffset(linePos - viewportHeight + 6)
	}
}

// Validation

// Validate validates all sections and returns errors
func (f *Form) Validate() []string {
	f.errors = nil

	for _, s := range f.sections {
		for _, err := range s.Validate() {
			f.errors = append(f.errors, err.Error())
		}
	}

	f.showErrors = len(f.errors) > 0
	return f.errors
}

// HasErrors returns true if the form has validation errors
func (f *Form) HasErrors() bool {
	for _, s := range f.sections {
		if s.HasErrors() {
			return true
		}
	}
	return false
}

// IsDirty returns true if any field has changed
func (f *Form) IsDirty() bool {
	for _, s := range f.sections {
		if s.IsDirty() {
			return true
		}
	}
	return false
}

// Data collection

// GetData returns all field values from all sections
func (f *Form) GetData() map[string]interface{} {
	data := make(map[string]interface{})
	for _, s := range f.sections {
		for k, v := range s.GetData() {
			data[k] = v
		}
	}
	return data
}

// SetData populates fields from a map
func (f *Form) SetData(data map[string]interface{}) {
	for key, value := range data {
		if field := f.GetField(key); field != nil {
			field.SetValue(value)
		}
	}
}

// Update handles input messages
//
//nolint:gocognit // Form input handling - complexity 58
func (f *Form) Update(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// Pass to current section
		if section := f.FocusedSection(); section != nil {
			return section.Update(msg)
		}
		return nil
	}

	// Global form keys
	switch {
	case key.Matches(keyMsg, f.keys.Submit):
		// Validate and submit
		errors := f.Validate()
		if len(errors) == 0 {
			return func() tea.Msg {
				return FormSubmitMsg{Data: f.GetData()}
			}
		}
		return nil

	case key.Matches(keyMsg, f.keys.Cancel):
		return func() tea.Msg {
			return FormCancelMsg{}
		}

	case key.Matches(keyMsg, f.keys.Help):
		f.showHelp = !f.showHelp
		return nil

	case key.Matches(keyMsg, f.keys.Select):
		// Handle Enter key when focused on action bar
		if f.focusedOnActions {
			if f.actionIndex == 0 {
				// Submit button selected
				errors := f.Validate()
				if len(errors) == 0 {
					return func() tea.Msg {
						return FormSubmitMsg{Data: f.GetData()}
					}
				}
			} else {
				// Cancel button selected
				return func() tea.Msg {
					return FormCancelMsg{}
				}
			}
			return nil
		}
		// Delegate Enter to section when not on actions
		if section := f.FocusedSection(); section != nil {
			return section.Update(msg)
		}
		return nil

	case key.Matches(keyMsg, f.keys.Left):
		// Navigate left in action bar
		if f.focusedOnActions && f.actionIndex > 0 {
			f.actionIndex--
			return nil
		}
		// Delegate to section for text input cursor movement
		if section := f.FocusedSection(); section != nil {
			return section.Update(msg)
		}
		return nil

	case key.Matches(keyMsg, f.keys.Right):
		// Navigate right in action bar
		if f.focusedOnActions && f.actionIndex < 1 {
			f.actionIndex++
			return nil
		}
		// Delegate to section for text input cursor movement
		if section := f.FocusedSection(); section != nil {
			return section.Update(msg)
		}
		return nil

	case key.Matches(keyMsg, f.keys.Tab), key.Matches(keyMsg, f.keys.Down):
		// Navigate between action buttons with Tab when on actions
		if f.focusedOnActions {
			if f.actionIndex < 1 {
				f.actionIndex++
				return nil
			}
			// At end of actions, wrap or stay
			return nil
		}
		// Check if current field is capturing input (e.g., dropdown open)
		if section := f.FocusedSection(); section != nil {
			if field := section.FocusedField(); field != nil {
				if field.dropdownOpen {
					return section.Update(msg)
				}
			}
		}
		f.nextField()
		f.scrollToFocused()
		return nil

	case key.Matches(keyMsg, f.keys.ShiftTab), key.Matches(keyMsg, f.keys.Up):
		// Navigate between action buttons with Shift+Tab when on actions
		if f.focusedOnActions {
			if f.actionIndex > 0 {
				f.actionIndex--
				return nil
			}
			// At start of actions, go back to form
			f.prevField()
			f.scrollToFocused()
			return nil
		}
		if section := f.FocusedSection(); section != nil {
			if field := section.FocusedField(); field != nil {
				if field.dropdownOpen {
					return section.Update(msg)
				}
			}
		}
		f.prevField()
		f.scrollToFocused()
		return nil
	}

	// Delegate to current section
	if section := f.FocusedSection(); section != nil && !f.focusedOnActions {
		return section.Update(msg)
	}

	return nil
}

// View renders the form
func (f *Form) View() string {
	var b strings.Builder

	// Title
	b.WriteString(f.styles.Title.Render(f.Title))
	b.WriteString("\n")

	// Subtitle with mode indicator
	if f.Subtitle != "" {
		b.WriteString(f.styles.Subtitle.Render(f.Subtitle))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Error box if validation failed
	if f.showErrors && len(f.errors) > 0 {
		errorText := "Please fix the following errors:\n"
		for _, err := range f.errors {
			errorText += "  • " + err + "\n"
		}
		b.WriteString(f.styles.ErrorBox.Render(f.styles.Error.Render(errorText)))
		b.WriteString("\n")
	}

	// Sections
	var sectionsContent strings.Builder
	for _, section := range f.sections {
		sectionsContent.WriteString(section.View())
	}

	if f.useViewport && f.contentReady {
		f.viewport.SetContent(sectionsContent.String())
		b.WriteString(f.viewport.View())
	} else {
		b.WriteString(sectionsContent.String())
	}

	// Action bar
	b.WriteString(f.renderActionBar())

	// Help
	if f.showHelp {
		b.WriteString(f.renderHelp())
	} else {
		b.WriteString(f.styles.Help.Render("\nPress ? for help"))
	}

	return f.styles.Container.Render(b.String())
}

// renderActionBar renders the submit/cancel action bar
func (f *Form) renderActionBar() string {
	var b strings.Builder

	// Button styling based on focus and which button is selected
	submitStyle := f.styles.Button
	cancelStyle := f.styles.Button

	if f.focusedOnActions {
		if f.actionIndex == 0 {
			submitStyle = f.styles.ButtonFocus
		} else {
			cancelStyle = f.styles.ButtonFocus
		}
	}

	submitLabel := "Submit"
	switch f.Mode {
	case FormModeCreate:
		submitLabel = "Create"
	case FormModeEdit:
		submitLabel = "Save Changes"
	case FormModeClone:
		submitLabel = "Clone"
	}

	b.WriteString(submitStyle.Render("[Ctrl+S] " + submitLabel))
	b.WriteString("  ")
	b.WriteString(cancelStyle.Render("[Esc] Cancel"))

	return f.styles.ActionBar.Render(b.String())
}

// renderHelp renders the help section
func (f *Form) renderHelp() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(f.styles.Subtitle.Render("─── Keyboard Shortcuts ───"))
	b.WriteString("\n\n")

	helpItems := [][]string{
		{"Tab / ↓", "Next field"},
		{"Shift+Tab / ↑", "Previous field"},
		{"Enter / Space", "Select/toggle option"},
		{"Ctrl+S", "Submit form"},
		{"Esc", "Cancel"},
		{"?", "Toggle help"},
	}

	for _, item := range helpItems {
		b.WriteString("  ")
		b.WriteString(f.styles.Title.Render(item[0]))
		b.WriteString("  ")
		b.WriteString(f.styles.Help.Render(item[1]))
		b.WriteString("\n")
	}

	return b.String()
}

// ShortHelp returns a short help string
func (f *Form) ShortHelp() string {
	return "tab:next  shift+tab:prev  ctrl+s:submit  esc:cancel  ?:help"
}
