package views

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/forms"
	"github.com/slayer/gcon/internal/ui/context"
)

// createViewState represents the lifecycle states for resource creation views.
type createViewState int

const (
	createViewStateForm   createViewState = iota
	createViewStateSaving
)

// createViewKeyMap defines shared key bindings for creation views.
type createViewKeyMap struct {
	Cancel key.Binding
}

func defaultCreateViewKeyMap() createViewKeyMap {
	return createViewKeyMap{
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// CreateViewBase provides shared lifecycle for resource creation views.
// Embed in concrete creation views to avoid repeating state machine,
// spinner, form sizing, and error handling boilerplate.
type CreateViewBase struct {
	Form    *forms.Form
	Spinner spinner.Model
	State   createViewState
	Err     error
	Keys    createViewKeyMap
	Ctx     *context.ProgramContext
	Width   int
	Height  int

	// SavingMsg is the message shown during the saving state (e.g. "Creating snapshot...")
	SavingMsg string
}

// NewCreateViewBase creates a new base with standard GCP spinner and key map.
func NewCreateViewBase(savingMsg string) CreateViewBase {
	return CreateViewBase{
		Spinner:   components.NewGCPSpinner(),
		State:     createViewStateForm,
		Keys:      defaultCreateViewKeyMap(),
		SavingMsg: savingMsg,
	}
}

// Init delegates to the form's Init.
func (b *CreateViewBase) Init() tea.Cmd {
	if b.Form != nil {
		return b.Form.Init()
	}
	return nil
}

// IsSaving returns true if the view is in the saving state.
func (b *CreateViewBase) IsSaving() bool {
	return b.State == createViewStateSaving
}

// BeginSaving transitions to saving state and returns a spinner tick command.
func (b *CreateViewBase) BeginSaving() tea.Cmd {
	b.State = createViewStateSaving
	b.Err = nil
	return b.Spinner.Tick
}

// SetError resets the view to form state and stores the error.
func (b *CreateViewBase) SetError(err error) {
	b.State = createViewStateForm
	b.Err = err
}

// SetContext updates dimensions and propagates to the form.
func (b *CreateViewBase) SetContext(ctx *context.ProgramContext) {
	b.Ctx = ctx
	b.Width = ctx.ContentWidth
	b.Height = ctx.ContentHeight
	if b.Form != nil {
		b.Form.SetSize(ctx.ContentWidth-formWidthPadding, ctx.ContentHeight-formHeightPadding)
	}
}

// HasTextInputFocused delegates to the form.
func (b *CreateViewBase) HasTextInputFocused() bool {
	if b.Form != nil {
		return b.Form.HasTextInputFocused()
	}
	return false
}

// View renders the form with error, or the saving spinner.
func (b *CreateViewBase) View() string {
	if b.State == createViewStateSaving {
		return renderSaving(b.Spinner, b.SavingMsg)
	}

	content := ""
	if b.Form != nil {
		content = b.Form.View()
	}

	if b.Err != nil {
		content += components.RenderInlineError(b.Err)
	}

	return content
}

// HandleBaseUpdate handles spinner ticks and cancel-during-saving.
// Returns (cmd, handled). If handled is true, the caller should return cmd.
// cancelMsg is the message to emit when the user cancels.
func (b *CreateViewBase) HandleBaseUpdate(msg tea.Msg, cancelMsg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if b.State == createViewStateSaving {
			var cmd tea.Cmd
			b.Spinner, cmd = b.Spinner.Update(msg)
			return cmd, true
		}
		return nil, true

	case tea.KeyMsg:
		// Allow cancel during saving, block other keys
		if b.State == createViewStateSaving {
			if key.Matches(msg, b.Keys.Cancel) {
				return func() tea.Msg { return cancelMsg }, true
			}
			return nil, true
		}
	}
	return nil, false
}

// UpdateForm delegates a message to the form. Call this as the final fallthrough.
func (b *CreateViewBase) UpdateForm(msg tea.Msg) tea.Cmd {
	if b.Form != nil {
		return b.Form.Update(msg)
	}
	return nil
}
