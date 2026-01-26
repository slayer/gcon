package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/ui/components/diff"
	"github.com/slayer/gcon/internal/ui/components/forms"
	"github.com/slayer/gcon/internal/ui/context"
)

// FormDemoState tracks what part of the demo we're viewing
type FormDemoState int

const (
	FormDemoStateForm FormDemoState = iota
	FormDemoStateDiff
	FormDemoStateDone
)

// FormDemoView demonstrates all form components for testing.
// Access via command palette with ":form-demo".
type FormDemoView struct {
	ctx *context.ProgramContext

	// Demo state
	state FormDemoState
	form  *forms.Form
	diff  *diff.Viewer

	// Collected data
	formData map[string]any

	// Styles
	styles formDemoStyles
}

type formDemoStyles struct {
	container lipgloss.Style
	title     lipgloss.Style
	success   lipgloss.Style
	muted     lipgloss.Style
}

func defaultFormDemoStyles() formDemoStyles {
	return formDemoStyles{
		container: lipgloss.NewStyle().Padding(1, 2),
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4285F4")),
		success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#34A853")),
		muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9AA0A6")),
	}
}

// NewFormDemoView creates a new form demo view
func NewFormDemoView() *FormDemoView {
	v := &FormDemoView{
		state:  FormDemoStateForm,
		styles: defaultFormDemoStyles(),
	}
	v.buildDemoForm()
	return v
}

// buildDemoForm creates the demo form with all field types
func (v *FormDemoView) buildDemoForm() {
	v.form = forms.NewForm("Form Components Demo", forms.FormModeCreate).
		SetSubtitle("Demonstrating all form field types and validation").
		EnableViewport()

	// Basic text fields section
	basicSection := forms.NewSection("basic", "Basic Text Fields").
		AddField(forms.NewTextField("name", "Name").
			SetRequired(true).
			SetPlaceholder("Enter your name").
			SetHelpText("This is a required text field").
			SetValidator(forms.ValidateStringLength(2, 50))).
		AddField(forms.NewTextField("email", "Email").
			SetPlaceholder("user@example.com").
			SetHelpText("Optional email address").
			SetValidator(forms.ValidateEmail())).
		AddField(forms.NewTextField("resource_name", "GCP Resource Name").
			SetRequired(true).
			SetPlaceholder("my-resource").
			SetHelpText("Lowercase letters, numbers, hyphens").
			SetValidator(forms.ComposeValidators(
				forms.ValidateRequired,
				forms.ValidateGCPResourceName,
			)))

	v.form.AddSection(basicSection)

	// Number fields section
	numberSection := forms.NewSection("numbers", "Number Fields").
		AddField(forms.NewNumberField("disk_size", "Disk Size (GB)").
			SetRequired(true).
			SetHelpText("10-1000 GB").
			SetValidator(forms.ValidateNumber(10, 1000))).
		AddField(forms.NewNumberField("replicas", "Replica Count").
			SetHelpText("0-10 replicas").
			SetValidator(forms.ValidateNumber(0, 10)))

	v.form.AddSection(numberSection)

	// Selection fields section
	selectionSection := forms.NewSection("selection", "Selection Fields").
		AddField(forms.NewDropdownField("zone", "Zone").
			SetRequired(true).
			SetOptionsFromStrings([]string{
				"us-central1-a",
				"us-central1-b",
				"us-east1-a",
				"us-east1-b",
				"europe-west1-a",
			}).
			SetHelpText("Select a GCP zone")).
		AddField(forms.NewDropdownField("machine_type", "Machine Type").
			SetOptions([]forms.Option{
				{Value: "e2-micro", Label: "e2-micro (0.25 vCPU, 1GB)", Description: "Shared-core, cost-optimized"},
				{Value: "e2-small", Label: "e2-small (0.5 vCPU, 2GB)", Description: "Shared-core, small workloads"},
				{Value: "e2-medium", Label: "e2-medium (1 vCPU, 4GB)", Description: "Shared-core, general purpose"},
				{Value: "n2-standard-2", Label: "n2-standard-2 (2 vCPU, 8GB)", Description: "Balanced compute"},
				{Value: "n2-standard-4", Label: "n2-standard-4 (4 vCPU, 16GB)", Description: "Balanced compute"},
			}).
			SetHelpText("Select machine type")).
		AddField(forms.NewMultiSelectField("network_tags", "Network Tags").
			SetOptionsFromStrings([]string{
				"http-server",
				"https-server",
				"allow-ssh",
				"allow-internal",
				"load-balanced",
			}).
			SetHelpText("Select firewall tags (multiple allowed)"))

	v.form.AddSection(selectionSection)

	// Toggle fields section
	toggleSection := forms.NewSection("toggles", "Toggle Fields").
		AddField(forms.NewToggleField("preemptible", "Preemptible").
			SetHelpText("Lower cost, may be preempted")).
		AddField(forms.NewToggleField("deletion_protection", "Deletion Protection").
			SetHelpText("Prevent accidental deletion"))

	v.form.AddSection(toggleSection)

	// Read-only fields section
	readonlySection := forms.NewSection("readonly", "Read-Only Fields").
		AddField(forms.NewReadOnlyField("status", "Status", "RUNNING")).
		AddField(forms.NewReadOnlyField("created", "Created At", "2026-01-18 10:30:00 UTC")).
		AddField(forms.NewReadOnlyField("fingerprint", "Fingerprint", "abc123def456"))

	v.form.AddSection(readonlySection)

	// Advanced section (collapsible)
	advancedSection := forms.NewSection("advanced", "Advanced Options").
		SetCollapsible(true).
		SetCollapsed(true).
		SetIcon("⚙").
		SetDescription("Optional advanced configuration").
		AddField(forms.NewTextField("metadata_key", "Metadata Key").
			SetPlaceholder("startup-script").
			SetValidator(forms.ValidateGCPLabelKey)).
		AddField(forms.NewTextAreaField("metadata_value", "Metadata Value").
			SetRows(3).
			SetPlaceholder("Enter metadata value...").
			SetHelpText("Value for the metadata key")).
		AddField(forms.NewTextAreaField("startup_script", "Startup Script").
			SetRows(6).
			SetShowLineNumbers(true).
			SetPlaceholder("#!/bin/bash\necho 'Hello World'").
			SetHelpText("Script to run on instance startup"))

	v.form.AddSection(advancedSection)
}

// SetContext updates the view context
func (v *FormDemoView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	if ctx != nil {
		v.form.SetSize(ctx.ContentWidth, ctx.ContentHeight-4)
	}
}

// Init initializes the view
func (v *FormDemoView) Init() tea.Cmd {
	v.form.Init()
	return nil
}

// HasTextInputFocused returns true if a text input field is focused in the form.
// Used by app to know whether to capture character keys or handle globally.
func (v *FormDemoView) HasTextInputFocused() bool {
	if v.state == FormDemoStateForm && v.form != nil {
		return v.form.HasTextInputFocused()
	}
	return false
}

// Update handles input
func (v *FormDemoView) Update(msg tea.Msg) tea.Cmd {
	switch v.state {
	case FormDemoStateForm:
		return v.updateForm(msg)
	case FormDemoStateDiff:
		return v.updateDiff(msg)
	case FormDemoStateDone:
		// Any key returns to form
		if _, ok := msg.(tea.KeyMsg); ok {
			v.state = FormDemoStateForm
			v.buildDemoForm()
		}
		return nil
	}
	return nil
}

func (v *FormDemoView) updateForm(msg tea.Msg) tea.Cmd {
	// Handle form result messages (come back from runtime after form returns them as commands)
	switch msg.(type) {
	case forms.FormSubmitMsg:
		// Form submitted - show diff
		v.formData = v.form.GetData()
		v.buildDiff()
		v.state = FormDemoStateDiff
		return nil
	case forms.FormCancelMsg:
		// Cancelled - reset form
		v.buildDemoForm()
		v.form.Init()
		return nil
	}

	// Pass all other messages to form - it returns commands for the runtime
	return v.form.Update(msg)
}

func (v *FormDemoView) updateDiff(msg tea.Msg) tea.Cmd {
	// Handle diff result messages (come back from runtime)
	switch msg.(type) {
	case diff.ConfirmMsg:
		// Confirmed - show success
		v.state = FormDemoStateDone
		return nil
	case diff.CancelMsg:
		// Cancelled - back to form
		v.state = FormDemoStateForm
		return nil
	}

	// Pass all other messages to diff viewer
	return v.diff.Update(msg)
}

// buildDiff creates a diff viewer from form data
func (v *FormDemoView) buildDiff() {
	fields := []diff.Field{
		{Label: "Name", OldValue: "", NewValue: v.getStringValue("name")},
		{Label: "Email", OldValue: "", NewValue: v.getStringValue("email")},
		{Label: "Resource Name", OldValue: "", NewValue: v.getStringValue("resource_name")},
		{Label: "Disk Size", OldValue: "", NewValue: v.getStringValue("disk_size") + " GB"},
		{Label: "Zone", OldValue: "", NewValue: v.getStringValue("zone")},
		{Label: "Machine Type", OldValue: "", NewValue: v.getStringValue("machine_type")},
		{Label: "Network Tags", OldValue: "", NewValue: v.getStringValue("network_tags")},
		{Label: "Preemptible", OldValue: "", NewValue: v.getStringValue("preemptible")},
		{Label: "Deletion Protection", OldValue: "", NewValue: v.getStringValue("deletion_protection")},
		{Label: "Metadata Key", OldValue: "", NewValue: v.getStringValue("metadata_key")},
		{Label: "Metadata Value", OldValue: "", NewValue: v.getStringValue("metadata_value")},
		{Label: "Startup Script", OldValue: "", NewValue: v.getStringValue("startup_script")},
	}

	v.diff = diff.New("Confirm Form Submission", fields)
	v.diff.SetWarnings([]string{
		"This is a demo - no actual resources will be created",
	})

	width := 60
	if v.ctx != nil {
		width = v.ctx.ContentWidth - 4
	}
	v.diff.SetSize(width, 0)
}

// getStringValue safely gets a string representation of a form value
func (v *FormDemoView) getStringValue(key string) string {
	val, ok := v.formData[key]
	if !ok {
		return ""
	}

	switch value := val.(type) {
	case string:
		if value == "" {
			return "(not set)"
		}
		return value
	case int64:
		return fmt.Sprintf("%d", value)
	case bool:
		if value {
			return "Yes"
		}
		return "No"
	case []string:
		if len(value) == 0 {
			return "(none)"
		}
		return strings.Join(value, ", ")
	default:
		return fmt.Sprintf("%v", val)
	}
}

// View renders the demo
func (v *FormDemoView) View() string {
	switch v.state {
	case FormDemoStateForm:
		return v.form.View()

	case FormDemoStateDiff:
		return v.diff.View()

	case FormDemoStateDone:
		return v.renderDone()
	}

	return ""
}

func (v *FormDemoView) renderDone() string {
	var b strings.Builder

	b.WriteString(v.styles.title.Render("✓ Form Demo Complete"))
	b.WriteString("\n\n")

	b.WriteString(v.styles.success.Render("Form data was successfully collected and displayed!"))
	b.WriteString("\n\n")

	b.WriteString(v.styles.muted.Render("Collected values:"))
	b.WriteString("\n\n")

	for key, val := range v.formData {
		b.WriteString(fmt.Sprintf("  %s: %v\n", key, val))
	}

	b.WriteString("\n")
	b.WriteString(v.styles.muted.Render("Press any key to restart the demo..."))

	return v.styles.container.Render(b.String())
}
