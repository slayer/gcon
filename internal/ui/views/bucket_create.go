package views

import (
	gocontext "context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/components/diff"
	"github.com/slayer/gcon/internal/ui/components/forms"
	"github.com/slayer/gcon/internal/ui/context"
)

// BucketCreateRequestMsg requests opening the bucket create view
type BucketCreateRequestMsg struct {
	ProjectID string
}

// BucketCreatedMsg indicates successful bucket creation
type BucketCreatedMsg struct {
	BucketName string
}

// BucketCreateCanceledMsg indicates user canceled bucket creation
type BucketCreateCanceledMsg struct{}

// bucketCreateState represents the view's state machine
type bucketCreateState int

const (
	bucketCreateStateForm bucketCreateState = iota
	bucketCreateStateDiff
	bucketCreateStateSaving
	bucketCreateStateError
)

// Internal message types
type bucketCreateSuccessMsg struct{}
type bucketCreateErrorMsg struct{ err error }

// bucketCreateKeyMap defines key bindings for the view
type bucketCreateKeyMap struct {
	Save    key.Binding
	Cancel  key.Binding
	Refresh key.Binding
}

func defaultBucketCreateKeyMap() bucketCreateKeyMap {
	return bucketCreateKeyMap{
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "preview changes"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "retry"),
		),
	}
}

// BucketCreateView allows creating new GCS buckets
type BucketCreateView struct {
	storageClient *gcp.StorageClient
	projectID     string
	ctx           *context.ProgramContext

	// State machine
	state bucketCreateState

	// UI components
	form       *forms.Form
	diffViewer *diff.Viewer
	spinner    spinner.Model
	err        error
	width      int
	height     int
	keys       bucketCreateKeyMap
}

// NewBucketCreateView creates a new bucket create view
func NewBucketCreateView(projectID string, storageClient *gcp.StorageClient) *BucketCreateView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))

	v := &BucketCreateView{
		storageClient: storageClient,
		projectID:     projectID,
		spinner:       s,
		state:         bucketCreateStateForm,
		keys:          defaultBucketCreateKeyMap(),
	}

	v.buildForm()
	return v
}

// buildForm creates the bucket creation form
func (v *BucketCreateView) buildForm() {
	v.form = forms.NewForm("Create Bucket", forms.FormModeCreate).
		SetSubtitle("Create a new Cloud Storage bucket").
		EnableViewport()

	// Basic Settings section
	basicSection := forms.NewSection("basic", "Basic Settings").
		AddField(forms.NewTextField("name", "Bucket Name").
			SetRequired(true).
			SetPlaceholder("my-bucket-name").
			SetHelpText("Globally unique name (3-63 chars, lowercase, no 'goog' prefix)").
			SetValidator(forms.ComposeValidators(
				forms.ValidateRequired,
				forms.ValidateGCSBucketName,
			))).
		AddField(forms.NewDropdownField("location_type", "Location Type").
			SetRequired(true).
			SetOptionsFromStrings([]string{"region", "dual-region", "multi-region"}).
			SetHelpText("Choose how data is distributed geographically")).
		AddField(forms.NewDropdownField("location", "Location").
			SetRequired(true).
			SetOptionsFromStrings(gcp.GCSRegions).
			SetHelpText("Geographic location for your data")).
		AddField(forms.NewDropdownField("storage_class", "Storage Class").
			SetRequired(true).
			SetOptionsFromStrings(gcp.GCSStorageClasses).
			SetHelpText("STANDARD for frequent access, NEARLINE/COLDLINE/ARCHIVE for infrequent"))

	v.form.AddSection(basicSection)

	// Access Control section
	accessSection := forms.NewSection("access", "Access Control").
		AddField(forms.NewToggleField("public_access_prevention", "Public Access Prevention").
			SetValue(true).
			SetHelpText("Enforces that objects cannot be made public")).
		AddField(forms.NewDropdownField("access_control", "Access Control").
			SetOptionsFromStrings([]string{"Uniform", "Fine-grained"}).
			SetHelpText("Uniform: bucket-level ACLs only; Fine-grained: object-level ACLs"))

	v.form.AddSection(accessSection)

	// Data Protection section
	protectionSection := forms.NewSection("protection", "Data Protection").
		AddField(forms.NewToggleField("versioning", "Object Versioning").
			SetHelpText("Keep multiple versions of objects")).
		AddField(forms.NewNumberField("retention_days", "Retention Period (days)").
			SetPlaceholder("0").
			SetHelpText("Minimum retention period (0 = disabled, max 36500)").
			SetValidator(forms.ValidateNumber(0, 36500))).
		AddField(forms.NewNumberField("soft_delete_days", "Soft Delete Retention (days)").
			SetPlaceholder("7").
			SetHelpText("Days to retain deleted objects (0-90, default 7)").
			SetValidator(forms.ValidateNumber(0, 90)))

	v.form.AddSection(protectionSection)

	// Advanced section (collapsible)
	advancedSection := forms.NewSection("advanced", "Advanced").
		SetCollapsible(true).
		SetCollapsed(true).
		SetIcon("").
		SetDescription("Labels and encryption settings").
		AddField(forms.NewTextAreaField("labels_raw", "Labels").
			SetRows(4).
			SetPlaceholder("key1=value1\nkey2=value2").
			SetHelpText("One label per line in key=value format")).
		AddField(forms.NewTextField("encryption_key", "Customer-Managed Encryption Key").
			SetPlaceholder("projects/PROJECT/locations/LOCATION/keyRings/KEY_RING/cryptoKeys/KEY").
			SetHelpText("Optional CMEK for data encryption at rest"))

	v.form.AddSection(advancedSection)
}

// Init initializes the view
func (v *BucketCreateView) Init() tea.Cmd {
	v.form.Init()
	return nil
}

// SetContext updates the view with shared program context
func (v *BucketCreateView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

// applySize applies dimensions to child components
func (v *BucketCreateView) applySize(width, height int) {
	v.width = width
	v.height = height

	if v.form != nil {
		v.form.SetSize(width-4, height-8)
	}
	if v.diffViewer != nil {
		v.diffViewer.SetSize(width-8, height-10)
	}
}

// HasTextInputFocused returns true if a text input field is focused
func (v *BucketCreateView) HasTextInputFocused() bool {
	if v.state == bucketCreateStateForm && v.form != nil {
		return v.form.HasTextInputFocused()
	}
	return false
}

// Update handles messages
func (v *BucketCreateView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case forms.FormSubmitMsg:
		return v.showDiffPreview()

	case forms.FormCancelMsg:
		return func() tea.Msg { return BucketCreateCanceledMsg{} }

	case diff.ConfirmMsg:
		v.state = bucketCreateStateSaving
		v.err = nil
		return tea.Batch(v.spinner.Tick, v.createBucket())

	case diff.CancelMsg:
		v.state = bucketCreateStateForm
		return nil

	case bucketCreateSuccessMsg:
		data := v.form.GetData()
		bucketName, _ := data["name"].(string) //nolint:errcheck // Type assertion checked implicitly
		return func() tea.Msg {
			return BucketCreatedMsg{BucketName: bucketName}
		}

	case bucketCreateErrorMsg:
		v.state = bucketCreateStateError
		v.err = msg.err
		return nil

	case spinner.TickMsg:
		if v.state == bucketCreateStateSaving {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		return v.handleKeyMsg(msg)
	}

	return nil
}

// handleKeyMsg handles key presses
func (v *BucketCreateView) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	// Allow cancel during saving
	if v.state == bucketCreateStateSaving {
		if key.Matches(msg, v.keys.Cancel) {
			return func() tea.Msg { return BucketCreateCanceledMsg{} }
		}
		return nil
	}

	// Handle error state
	if v.state == bucketCreateStateError {
		if key.Matches(msg, v.keys.Refresh) {
			v.state = bucketCreateStateForm
			v.err = nil
			return nil
		}
		if key.Matches(msg, v.keys.Cancel) {
			return func() tea.Msg { return BucketCreateCanceledMsg{} }
		}
		return nil
	}

	// Handle diff state
	if v.state == bucketCreateStateDiff {
		return v.diffViewer.Update(msg)
	}

	// Handle form state - check for location type changes
	if v.state == bucketCreateStateForm {
		// Delegate to form and check for field changes
		cmd := v.form.Update(msg)

		// Update location options when location_type changes
		v.updateLocationOptions()

		return cmd
	}

	return nil
}

// updateLocationOptions updates the location dropdown based on selected location type
func (v *BucketCreateView) updateLocationOptions() {
	data := v.form.GetData()
	locationType, ok := data["location_type"].(string)
	if !ok {
		return
	}

	locationField := v.form.GetField("location")
	if locationField == nil {
		return
	}

	locations := gcp.GetLocationsForType(locationType)
	locationField.SetOptionsFromStrings(locations)

	// Reset to first option if current value isn't in new list
	currentLocation, _ := data["location"].(string) //nolint:errcheck // Type assertion checked implicitly
	found := false
	for _, loc := range locations {
		if loc == currentLocation {
			found = true
			break
		}
	}
	if !found && len(locations) > 0 {
		locationField.SetValue(locations[0])
	}
}

// showDiffPreview shows the confirmation diff view
func (v *BucketCreateView) showDiffPreview() tea.Cmd {
	data := v.form.GetData()

	// Build diff fields
	fields := []diff.Field{
		{Label: "Bucket Name", OldValue: "", NewValue: v.getStringValue(data, "name")},
		{Label: "Location Type", OldValue: "", NewValue: v.getStringValue(data, "location_type")},
		{Label: "Location", OldValue: "", NewValue: v.getStringValue(data, "location")},
		{Label: "Storage Class", OldValue: "", NewValue: v.getStringValue(data, "storage_class")},
		{Label: "Public Access Prevention", OldValue: "", NewValue: v.getBoolValue(data, "public_access_prevention")},
		{Label: "Access Control", OldValue: "", NewValue: v.getStringValue(data, "access_control")},
		{Label: "Object Versioning", OldValue: "", NewValue: v.getBoolValue(data, "versioning")},
		{Label: "Retention Period", OldValue: "", NewValue: v.getDaysValue(data, "retention_days")},
		{Label: "Soft Delete Retention", OldValue: "", NewValue: v.getDaysValue(data, "soft_delete_days")},
	}

	// Add labels if present
	labelsRaw := v.getStringValue(data, "labels_raw")
	if labelsRaw != "" && labelsRaw != "(not set)" {
		fields = append(fields, diff.Field{
			Label:    "Labels",
			OldValue: "",
			NewValue: labelsRaw,
		})
	}

	// Add encryption key if present
	encKey := v.getStringValue(data, "encryption_key")
	if encKey != "" && encKey != "(not set)" {
		fields = append(fields, diff.Field{
			Label:    "Encryption Key",
			OldValue: "",
			NewValue: encKey,
		})
	}

	v.diffViewer = diff.New("Confirm Bucket Creation", fields)
	v.diffViewer.SetWarnings([]string{
		"Bucket names are globally unique and cannot be changed after creation",
		"Some settings (location, storage class) cannot be changed after creation",
	})
	v.diffViewer.SetSize(v.width-8, v.height-10)
	v.state = bucketCreateStateDiff

	return nil
}

// getStringValue safely gets a string value
func (v *BucketCreateView) getStringValue(data map[string]any, key string) string {
	val, ok := data[key]
	if !ok {
		return "(not set)"
	}
	if str, ok := val.(string); ok && str != "" {
		return str
	}
	return "(not set)"
}

// getBoolValue converts a bool to Yes/No
func (v *BucketCreateView) getBoolValue(data map[string]any, key string) string {
	val, ok := data[key]
	if !ok {
		return "No"
	}
	if b, ok := val.(bool); ok && b {
		return "Yes"
	}
	return "No"
}

// getDaysValue formats a days value
func (v *BucketCreateView) getDaysValue(data map[string]any, key string) string {
	val, ok := data[key]
	if !ok {
		return "Disabled"
	}
	switch n := val.(type) {
	case int64:
		if n == 0 {
			return "Disabled"
		}
		return fmt.Sprintf("%d days", n)
	case int:
		if n == 0 {
			return "Disabled"
		}
		return fmt.Sprintf("%d days", n)
	case string:
		if n == "" || n == "0" {
			return "Disabled"
		}
		return n + " days"
	}
	return "Disabled"
}

// createBucket creates the bucket in GCP
func (v *BucketCreateView) createBucket() tea.Cmd {
	return func() tea.Msg {
		if v.storageClient == nil {
			return bucketCreateErrorMsg{err: uierrors.ErrStorageClientNotInitialized}
		}

		data := v.form.GetData()
		config := v.buildConfig(data)

		ctx := gocontext.Background()
		if err := v.storageClient.CreateBucket(ctx, v.projectID, config); err != nil {
			return bucketCreateErrorMsg{err: err}
		}

		return bucketCreateSuccessMsg{}
	}
}

// buildConfig constructs a BucketCreateConfig from form data
func (v *BucketCreateView) buildConfig(data map[string]any) gcp.BucketCreateConfig {
	config := gcp.BucketCreateConfig{
		Name:         v.getString(data, "name"),
		Location:     v.getString(data, "location"),
		LocationType: v.getString(data, "location_type"),
		StorageClass: v.getString(data, "storage_class"),
	}

	// Access control
	config.PublicAccessPrevention = v.getBool(data, "public_access_prevention")
	accessControl := v.getString(data, "access_control")
	config.UniformBucketAccess = accessControl == "Uniform"

	// Data protection
	config.VersioningEnabled = v.getBool(data, "versioning")
	config.RetentionDays = v.getInt(data, "retention_days")
	config.SoftDeleteDays = v.getInt(data, "soft_delete_days")

	// Parse labels from raw text
	labelsRaw := v.getString(data, "labels_raw")
	if labelsRaw != "" {
		config.Labels = v.parseLabels(labelsRaw)
	}

	// Encryption key
	config.EncryptionKey = v.getString(data, "encryption_key")

	return config
}

// Helper methods for extracting typed values
func (v *BucketCreateView) getString(data map[string]any, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (v *BucketCreateView) getBool(data map[string]any, key string) bool {
	if val, ok := data[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func (v *BucketCreateView) getInt(data map[string]any, key string) int {
	if val, ok := data[key]; ok {
		switch n := val.(type) {
		case int64:
			return int(n)
		case int:
			return n
		case float64:
			return int(n)
		}
	}
	return 0
}

// parseLabels converts key=value lines into a map
func (v *BucketCreateView) parseLabels(raw string) map[string]string {
	labels := make(map[string]string)
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" {
				labels[key] = value
			}
		}
	}
	return labels
}

// View renders the view
func (v *BucketCreateView) View() string {
	switch v.state {
	case bucketCreateStateSaving:
		return v.renderLoading("Creating bucket...")

	case bucketCreateStateError:
		return v.renderError()

	case bucketCreateStateDiff:
		return v.renderDiff()

	case bucketCreateStateForm:
		return v.form.View()
	}

	return v.renderLoading("Initializing...")
}

// renderLoading renders a loading state
func (v *BucketCreateView) renderLoading(msg string) string {
	return fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
}

// renderError renders an error state
func (v *BucketCreateView) renderError() string {
	var b strings.Builder

	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	b.WriteString("\n")
	b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %v", v.err)))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  r: retry  esc: go back"))
	b.WriteString("\n")

	return b.String()
}

// renderDiff renders the diff confirmation view
func (v *BucketCreateView) renderDiff() string {
	if v.diffViewer == nil {
		return v.renderLoading("Preparing preview...")
	}

	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	b.WriteString(titleStyle.Render("Create Bucket"))
	b.WriteString("\n\n")

	b.WriteString(v.diffViewer.View())

	return b.String()
}

// GetStorageClient returns the storage client
func (v *BucketCreateView) GetStorageClient() *gcp.StorageClient {
	return v.storageClient
}
