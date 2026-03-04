package views

import (
	gocontext "context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/diff"
	"github.com/slayer/gcon/internal/ui/components/forms"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
)

// cloudRunEditState represents the view lifecycle
type cloudRunEditState int

const (
	cloudRunEditStateLoading cloudRunEditState = iota // Edit mode: fetching current config
	cloudRunEditStateForm                             // User edits fields
	cloudRunEditStateDiff                             // Preview old vs new
	cloudRunEditStateSaving                           // Calling API
)

var (
	errInvalidServiceName = errors.New("invalid service name type")
	errInvalidRegion      = errors.New("invalid region type")
)

// Internal messages
type crEditLoadedMsg struct{ details *gcp.CloudRunServiceDetails }
type crEditLoadErrorMsg struct{ err error }
type crEditSaveSuccessMsg struct{}
type crEditSaveErrorMsg struct{ err error }

// cloudRunEditKeyMap defines key bindings for the edit view
type cloudRunEditKeyMap struct {
	Cancel  key.Binding
	Retry   key.Binding
	Refresh key.Binding
}

func defaultCloudRunEditKeyMap() cloudRunEditKeyMap {
	return cloudRunEditKeyMap{
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel/back"),
		),
		Retry: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "retry"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// CloudRunEditView handles editing an existing or creating a new Cloud Run service.
type CloudRunEditView struct {
	runClient   *gcp.CloudRunClient
	projectID   string
	serviceName string // Empty for create mode
	fullName    string // Empty for create mode
	isCreate    bool
	ctx         *context.ProgramContext

	state    cloudRunEditState
	original *gcp.CloudRunServiceDetails // nil for create mode

	form       *forms.Form
	diffViewer *diff.Viewer
	spinner    spinner.Model
	err        error
	keys       cloudRunEditKeyMap

	width, height int
}

// NewCloudRunEditView creates a new Cloud Run edit/create view.
func NewCloudRunEditView(projectID, serviceName, fullName string, runClient *gcp.CloudRunClient, isCreate bool) *CloudRunEditView {
	v := &CloudRunEditView{
		runClient:   runClient,
		projectID:   projectID,
		serviceName: serviceName,
		fullName:    fullName,
		isCreate:    isCreate,
		spinner:     components.NewGCPSpinner(),
		keys:        defaultCloudRunEditKeyMap(),
	}

	if isCreate {
		v.state = cloudRunEditStateForm
		v.buildForm()
	} else {
		v.state = cloudRunEditStateLoading
	}

	return v
}

// IsCreate returns true if this is a create (not edit) view.
func (v *CloudRunEditView) IsCreate() bool {
	return v.isCreate
}

// Init starts loading (edit mode) or initializes the form (create mode).
func (v *CloudRunEditView) Init() tea.Cmd {
	if v.isCreate {
		v.state = cloudRunEditStateForm
		v.err = nil
		if v.form != nil {
			return v.form.Init()
		}
		return nil
	}

	// Edit mode: load current service config
	v.state = cloudRunEditStateLoading
	v.err = nil
	return tea.Batch(v.spinner.Tick, v.loadService())
}

func (v *CloudRunEditView) loadService() tea.Cmd {
	return func() tea.Msg {
		if v.runClient == nil {
			return crEditLoadErrorMsg{err: uierrors.ErrCloudRunClientNotInitialized}
		}
		details, err := v.runClient.GetService(gocontext.Background(), v.fullName)
		if err != nil {
			return crEditLoadErrorMsg{err: err}
		}
		return crEditLoadedMsg{details: details}
	}
}

// Update handles messages for the edit view.
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern requires centralized message handling
func (v *CloudRunEditView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case crEditLoadedMsg:
		v.original = msg.details
		v.buildForm()
		v.populateFormFromOriginal()
		v.state = cloudRunEditStateForm
		v.applySize()
		if v.form != nil {
			return v.form.Init()
		}
		return nil

	case crEditLoadErrorMsg:
		v.state = cloudRunEditStateForm
		v.err = msg.err
		return nil

	case crEditSaveSuccessMsg:
		action := "edit"
		name := v.serviceName
		if v.isCreate {
			action = "create"
			if v.form != nil {
				data := v.form.GetData()
				if n, ok := data["name"].(string); ok {
					name = n
				}
			}
		}
		return func() tea.Msg {
			return CloudRunEditResultMsg{
				Name:    name,
				Action:  action,
				Success: true,
			}
		}

	case crEditSaveErrorMsg:
		v.state = cloudRunEditStateForm
		v.err = msg.err
		return nil

	case diff.ConfirmMsg:
		v.state = cloudRunEditStateSaving
		v.err = nil
		return tea.Batch(v.spinner.Tick, v.save())

	case diff.CancelMsg:
		v.state = cloudRunEditStateForm
		return nil

	case forms.FormSubmitMsg:
		return v.showDiffPreview()

	case forms.FormCancelMsg:
		return func() tea.Msg { return CloudRunEditCanceledMsg{} }

	case spinner.TickMsg:
		if v.state == cloudRunEditStateLoading || v.state == cloudRunEditStateSaving {
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

func (v *CloudRunEditView) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	// Allow cancel during loading/saving
	if v.state == cloudRunEditStateLoading || v.state == cloudRunEditStateSaving {
		if key.Matches(msg, v.keys.Cancel) {
			return func() tea.Msg { return CloudRunEditCanceledMsg{} }
		}
		return nil
	}

	// Error state: retry or cancel
	if v.err != nil && v.state == cloudRunEditStateForm && v.original == nil && !v.isCreate {
		if key.Matches(msg, v.keys.Retry) {
			v.state = cloudRunEditStateLoading
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.loadService())
		}
		if key.Matches(msg, v.keys.Cancel) {
			return func() tea.Msg { return CloudRunEditCanceledMsg{} }
		}
		return nil
	}

	// Diff state: delegate to diff viewer
	if v.state == cloudRunEditStateDiff {
		return v.diffViewer.Update(msg)
	}

	// Form state: delegate to form
	if v.state == cloudRunEditStateForm && v.form != nil {
		return v.form.Update(msg)
	}

	return nil
}

// View renders the edit view.
func (v *CloudRunEditView) View() string {
	switch v.state {
	case cloudRunEditStateLoading:
		return renderLoading(v.spinner, "Loading service configuration...")

	case cloudRunEditStateSaving:
		if v.isCreate {
			return renderSaving(v.spinner, "Creating Cloud Run service...")
		}
		return renderSaving(v.spinner, "Updating Cloud Run service...")

	case cloudRunEditStateDiff:
		if v.diffViewer != nil {
			return v.diffViewer.View()
		}

	case cloudRunEditStateForm:
		// Error while loading (no form available yet)
		if v.err != nil && v.form == nil {
			return "\n" + components.RenderError(v.err)
		}

		content := ""
		if v.form != nil {
			content = v.form.View()
		}

		if v.err != nil {
			content += components.RenderInlineError(v.err)
		}

		return content
	}

	return ""
}

// SetContext updates the view with shared program context.
func (v *CloudRunEditView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize()
}

// SetError allows the app to propagate async errors back to the view.
func (v *CloudRunEditView) SetError(err error) {
	v.state = cloudRunEditStateForm
	v.err = err
}

// HasTextInputFocused returns true if a text input is active.
func (v *CloudRunEditView) HasTextInputFocused() bool {
	if v.state == cloudRunEditStateForm && v.form != nil {
		return v.form.HasTextInputFocused()
	}
	return false
}

func (v *CloudRunEditView) applySize() {
	if v.form != nil {
		v.form.SetSize(v.width-formWidthPadding, v.height-formHeightPadding)
	}
	if v.diffViewer != nil {
		v.diffViewer.SetSize(v.width-8, v.height-10)
	}
}

// buildForm creates the form for editing/creating a Cloud Run service.
func (v *CloudRunEditView) buildForm() {
	title := "Edit Cloud Run Service"
	mode := forms.FormModeEdit
	if v.isCreate {
		title = "Create Cloud Run Service"
		mode = forms.FormModeCreate
	}

	v.form = forms.NewForm(title, mode).EnableViewport()

	// Section 1: Service
	serviceSection := forms.NewSection("service", "Service")
	if v.isCreate {
		serviceSection.AddField(forms.NewTextField("name", "Name").
			SetRequired(true).
			SetPlaceholder("my-service").
			SetHelpText("Lowercase letters, numbers, and hyphens (1-63 chars)").
			SetValidator(forms.ComposeValidators(
				forms.ValidateRequired,
				forms.ValidateGCPResourceName,
			)))
		serviceSection.AddField(forms.NewDropdownField("region", "Region").
			SetRequired(true).
			SetOptionsFromStrings(cloudRunRegions()).
			SetHelpText("Region where the service will run"))
	} else {
		serviceSection.AddField(forms.NewReadOnlyField("name", "Name", v.serviceName))
		serviceSection.AddField(forms.NewReadOnlyField("region", "Region", extractRegionFromFullName(v.fullName)))
	}
	serviceSection.AddField(forms.NewTextField("description", "Description").
		SetPlaceholder("Service description").
		SetCharLimit(512))
	v.form.AddSection(serviceSection)

	// Section 2: Container
	containerSection := forms.NewSection("container", "Container")
	containerSection.AddField(forms.NewTextField("image", "Image").
		SetRequired(true).
		SetPlaceholder("gcr.io/project/image:tag").
		SetHelpText("Container image URL"))
	containerSection.AddField(forms.NewNumberField("port", "Port").
		SetPlaceholder("8080").
		SetHelpText("Container port (1-65535)").
		SetValidator(forms.ValidateNumber(1, 65535)))
	containerSection.AddField(forms.NewTextField("command", "Command").
		SetPlaceholder("/bin/sh -c").
		SetHelpText("Entrypoint override (space-separated)"))
	containerSection.AddField(forms.NewTextField("args", "Arguments").
		SetPlaceholder("--flag value").
		SetHelpText("Container arguments (space-separated)"))
	v.form.AddSection(containerSection)

	// Section 3: Environment Variables
	envSection := forms.NewSection("env", "Environment Variables").
		SetCollapsible(true)
	envSection.AddField(forms.NewTextAreaField("env_vars", "Env Vars").
		SetPlaceholder("KEY=value\nANOTHER=value").
		SetRows(6).
		SetHelpText("One KEY=value per line. Secret refs are preserved and shown as hints."))
	v.form.AddSection(envSection)

	// Section 4: Resources
	resourcesSection := forms.NewSection("resources", "Resources")
	resourcesSection.AddField(forms.NewDropdownField("cpu", "CPU").
		SetRequired(true).
		SetOptionsFromStrings([]string{"1", "2", "4", "6", "8"}).
		SetHelpText("Number of vCPUs"))
	resourcesSection.AddField(forms.NewDropdownField("memory", "Memory").
		SetRequired(true).
		SetOptionsFromStrings([]string{"128Mi", "256Mi", "512Mi", "1Gi", "2Gi", "4Gi", "8Gi", "16Gi", "32Gi"}).
		SetHelpText("Memory allocation"))
	resourcesSection.AddField(forms.NewNumberField("timeout", "Request Timeout (seconds)").
		SetPlaceholder("300").
		SetHelpText("Max time per request (1-3600s)").
		SetValidator(forms.ValidateNumber(1, 3600)))
	v.form.AddSection(resourcesSection)

	// Section 5: Scaling
	scalingSection := forms.NewSection("scaling", "Scaling")
	scalingSection.AddField(forms.NewNumberField("min_instances", "Min Instances").
		SetPlaceholder("0").
		SetHelpText("Minimum instance count (0 = scale to zero)").
		SetValidator(forms.ValidateNumber(0, 1000)))
	scalingSection.AddField(forms.NewNumberField("max_instances", "Max Instances").
		SetPlaceholder("100").
		SetHelpText("Maximum instance count (1-1000)").
		SetValidator(forms.ValidateNumber(1, 1000)))
	scalingSection.AddField(forms.NewNumberField("concurrency", "Max Concurrent Requests").
		SetPlaceholder("80").
		SetHelpText("Max requests per instance (1-1000)").
		SetValidator(forms.ValidateNumber(1, 1000)))
	v.form.AddSection(scalingSection)

	// Section 6: Networking (collapsible, collapsed)
	netSection := forms.NewSection("networking", "Networking").
		SetCollapsible(true).
		SetCollapsed(true).
		SetDescription("Ingress and VPC connector settings")
	netSection.AddField(forms.NewDropdownField("ingress", "Ingress").
		SetRequired(true).
		SetOptions([]forms.Option{
			{Value: "INGRESS_TRAFFIC_ALL", Label: "All"},
			{Value: "INGRESS_TRAFFIC_INTERNAL_ONLY", Label: "Internal only"},
			{Value: "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER", Label: "Internal + Load Balancer"},
			{Value: "INGRESS_TRAFFIC_NONE", Label: "None"},
		}))
	netSection.AddField(forms.NewTextField("vpc_connector", "VPC Connector").
		SetPlaceholder("projects/PROJECT/locations/REGION/connectors/NAME").
		SetHelpText("Full resource path (empty = none)"))
	netSection.AddField(forms.NewDropdownField("vpc_egress", "VPC Egress").
		SetOptions([]forms.Option{
			{Value: "", Label: "(default)"},
			{Value: "PRIVATE_RANGES_ONLY", Label: "Private ranges only"},
			{Value: "ALL_TRAFFIC", Label: "All traffic"},
		}))
	v.form.AddSection(netSection)

	// Section 7: Security (collapsible, collapsed)
	secSection := forms.NewSection("security", "Security").
		SetCollapsible(true).
		SetCollapsed(true).
		SetDescription("Service account configuration")
	secSection.AddField(forms.NewTextField("service_account", "Service Account").
		SetPlaceholder("sa@project.iam.gserviceaccount.com").
		SetHelpText("Email of the service account (empty = default)"))
	v.form.AddSection(secSection)

	// Set default values for create mode
	if v.isCreate {
		v.form.SetData(map[string]any{
			"port":          int64(8080),
			"cpu":           "1",
			"memory":        "512Mi",
			"timeout":       int64(300),
			"min_instances":  int64(0),
			"max_instances":  int64(100),
			"concurrency":   int64(80),
			"ingress":       "INGRESS_TRAFFIC_ALL",
		})
	}
}

// populateFormFromOriginal fills the form with current service values (edit mode).
func (v *CloudRunEditView) populateFormFromOriginal() {
	if v.original == nil || v.form == nil {
		return
	}

	d := v.original
	data := map[string]any{
		"description":     d.Description,
		"image":           d.ContainerImage,
		"port":            d.ContainerPort,
		"command":         strings.Join(d.Command, " "),
		"args":            strings.Join(d.Args, " "),
		"cpu":             d.CPU,
		"memory":          d.Memory,
		"timeout":         d.TimeoutSeconds,
		"min_instances":    d.MinInstances,
		"max_instances":    d.MaxInstances,
		"concurrency":     d.Concurrency,
		"ingress":         d.IngressRaw,
		"vpc_connector":   d.VPCConnector,
		"service_account": d.ServiceAccount,
	}

	// VPC egress: use raw API value directly (no reverse-mapping from display strings)
	data["vpc_egress"] = d.VPCEgressRaw

	// Format env vars as KEY=value text, noting secret refs
	data["env_vars"] = formatEnvVarsForEdit(d.EnvVars)

	v.form.SetData(data)
}

// formatEnvVarsForEdit formats env vars as KEY=value lines for the text area.
// Secret refs are excluded (they're preserved during save and shown as a hint).
func formatEnvVarsForEdit(envVars map[string]string) string {
	if len(envVars) == 0 {
		return ""
	}

	keys := make([]string, 0, len(envVars))
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	for _, k := range keys {
		v := envVars[k]
		if v == "(secret ref)" {
			continue // Secret refs are not editable
		}
		lines = append(lines, k+"="+v)
	}
	return strings.Join(lines, "\n")
}

// showDiffPreview builds a diff and transitions to the diff state.
func (v *CloudRunEditView) showDiffPreview() tea.Cmd {
	if errs := v.form.Validate(); len(errs) > 0 {
		return nil // Form displays validation errors
	}

	data := v.form.GetData()
	fields := v.buildDiffFields(data)

	title := "Deploy Changes"
	if v.isCreate {
		title = "Create Cloud Run Service"
	}

	v.diffViewer = diff.New(title, fields)

	// Warn about container changes triggering new revision (edit mode)
	if !v.isCreate && v.hasContainerChanges(data) {
		v.diffViewer.SetWarnings([]string{"Container changes will create a new revision"})
	}

	v.diffViewer.SetSize(v.width-8, v.height-10)
	v.state = cloudRunEditStateDiff
	return nil
}

// buildDiffFields creates diff fields from form data vs. original (or empty for create).
func (v *CloudRunEditView) buildDiffFields(data map[string]any) []diff.Field {
	getString := func(key string) string {
		if val, ok := data[key].(string); ok {
			return val
		}
		return ""
	}
	getInt64 := func(key string) string {
		switch val := data[key].(type) {
		case int64:
			return strconv.FormatInt(val, 10)
		case string:
			return val
		default:
			return ""
		}
	}

	old := v.original // nil for create mode
	oldStr := func(fn func(*gcp.CloudRunServiceDetails) string) string {
		if old == nil {
			return ""
		}
		return fn(old)
	}

	fields := []diff.Field{
		{Label: "Image", OldValue: oldStr(func(d *gcp.CloudRunServiceDetails) string { return d.ContainerImage }), NewValue: getString("image")},
		{Label: "Port", OldValue: oldStr(func(d *gcp.CloudRunServiceDetails) string { return strconv.FormatInt(d.ContainerPort, 10) }), NewValue: getInt64("port")},
		{Label: "CPU", OldValue: oldStr(func(d *gcp.CloudRunServiceDetails) string { return d.CPU }), NewValue: getString("cpu")},
		{Label: "Memory", OldValue: oldStr(func(d *gcp.CloudRunServiceDetails) string { return d.Memory }), NewValue: getString("memory")},
		{Label: "Description", OldValue: oldStr(func(d *gcp.CloudRunServiceDetails) string { return d.Description }), NewValue: getString("description")},
		{Label: "Min Instances", OldValue: oldStr(func(d *gcp.CloudRunServiceDetails) string { return strconv.FormatInt(d.MinInstances, 10) }), NewValue: getInt64("min_instances")},
		{Label: "Max Instances", OldValue: oldStr(func(d *gcp.CloudRunServiceDetails) string { return strconv.FormatInt(d.MaxInstances, 10) }), NewValue: getInt64("max_instances")},
		{Label: "Concurrency", OldValue: oldStr(func(d *gcp.CloudRunServiceDetails) string { return strconv.FormatInt(d.Concurrency, 10) }), NewValue: getInt64("concurrency")},
		{Label: "Timeout", OldValue: oldStr(func(d *gcp.CloudRunServiceDetails) string { return strconv.FormatInt(d.TimeoutSeconds, 10) }), NewValue: getInt64("timeout")},
		{Label: "Ingress", OldValue: oldStr(func(d *gcp.CloudRunServiceDetails) string { return d.IngressRaw }), NewValue: getString("ingress")},
		{Label: "Service Account", OldValue: oldStr(func(d *gcp.CloudRunServiceDetails) string { return d.ServiceAccount }), NewValue: getString("service_account")},
		{Label: "VPC Connector", OldValue: oldStr(func(d *gcp.CloudRunServiceDetails) string { return d.VPCConnector }), NewValue: getString("vpc_connector")},
	}

	// Command and args
	oldCmd := ""
	oldArgs := ""
	if old != nil {
		oldCmd = strings.Join(old.Command, " ")
		oldArgs = strings.Join(old.Args, " ")
	}
	fields = append(fields,
		diff.Field{Label: "Command", OldValue: oldCmd, NewValue: getString("command")},
		diff.Field{Label: "Arguments", OldValue: oldArgs, NewValue: getString("args")},
	)

	return fields
}

// hasContainerChanges checks if any container-level field changed from original.
func (v *CloudRunEditView) hasContainerChanges(data map[string]any) bool {
	if v.original == nil {
		return false
	}

	getString := func(key string) string {
		if val, ok := data[key].(string); ok {
			return val
		}
		return ""
	}
	getInt64 := func(key string) int64 {
		switch val := data[key].(type) {
		case int64:
			return val
		default:
			return 0
		}
	}

	d := v.original
	if getString("image") != d.ContainerImage {
		return true
	}
	if getInt64("port") != d.ContainerPort {
		return true
	}
	if getString("cpu") != d.CPU {
		return true
	}
	if getString("memory") != d.Memory {
		return true
	}
	if getString("command") != strings.Join(d.Command, " ") {
		return true
	}
	if getString("args") != strings.Join(d.Args, " ") {
		return true
	}
	return false
}

// save performs the actual update or create API call.
func (v *CloudRunEditView) save() tea.Cmd {
	return func() tea.Msg {
		if v.runClient == nil {
			return crEditSaveErrorMsg{err: uierrors.ErrCloudRunClientNotInitialized}
		}

		update := v.buildUpdate()
		ctx := gocontext.Background()

		if v.isCreate {
			data := v.form.GetData()
			name, ok := data["name"].(string)
			if !ok {
				return crEditSaveErrorMsg{err: errInvalidServiceName}
			}
			region, ok := data["region"].(string)
			if !ok {
				return crEditSaveErrorMsg{err: errInvalidRegion}
			}
			err := v.runClient.CreateService(ctx, v.projectID, region, name, update)
			if err != nil {
				return crEditSaveErrorMsg{err: err}
			}
			return crEditSaveSuccessMsg{}
		}

		err := v.runClient.UpdateService(ctx, v.fullName, update)
		if err != nil {
			return crEditSaveErrorMsg{err: err}
		}
		return crEditSaveSuccessMsg{}
	}
}

// buildUpdate constructs a CloudRunServiceUpdate from form data.
func (v *CloudRunEditView) buildUpdate() *gcp.CloudRunServiceUpdate {
	data := v.form.GetData()

	getString := func(key string) *string {
		if val, ok := data[key].(string); ok {
			return &val
		}
		s := ""
		return &s
	}
	getInt64 := func(key string) *int64 {
		switch val := data[key].(type) {
		case int64:
			return &val
		case string:
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				return &n
			}
		}
		n := int64(0)
		return &n
	}

	update := &gcp.CloudRunServiceUpdate{
		Description:    getString("description"),
		Ingress:        getString("ingress"),
		Image:          getString("image"),
		Port:           getInt64("port"),
		CPU:            getString("cpu"),
		Memory:         getString("memory"),
		MinInstances:   getInt64("min_instances"),
		MaxInstances:   getInt64("max_instances"),
		Concurrency:    getInt64("concurrency"),
		Timeout:        getInt64("timeout"),
		ServiceAccount: getString("service_account"),
	}

	// Command: split on spaces, nil means "don't change" but we always set from form
	if cmd := strings.TrimSpace(*getString("command")); cmd != "" {
		update.Command = strings.Fields(cmd)
	} else {
		update.Command = []string{} // Clear entrypoint
	}

	if args := strings.TrimSpace(*getString("args")); args != "" {
		update.Args = strings.Fields(args)
	} else {
		update.Args = []string{} // Clear args
	}

	// Env vars: parse from KEY=value text, merging with preserved secret refs
	update.EnvVars = v.buildEnvVars(data)

	// VPC access: only include if user actually changed values.
	// Sending VpcAccess without both connector/network_interfaces causes API errors
	// on services using Direct VPC egress (NetworkInterfaces instead of Connector).
	if vpc, ok := data["vpc_connector"].(string); ok {
		origConnector := ""
		if v.original != nil {
			origConnector = v.original.VPCConnector
		}
		if vpc != origConnector {
			update.VPCConnector = &vpc
		}
	}
	if egress, ok := data["vpc_egress"].(string); ok {
		origEgress := ""
		if v.original != nil {
			origEgress = v.original.VPCEgressRaw
		}
		if egress != origEgress {
			update.VPCEgress = &egress
		}
	}

	// Pass original container for safe merging (preserves probes, volume mounts, secret env vars)
	if v.original != nil {
		update.OriginalContainer = v.original.OriginalContainer
	}

	return update
}

// buildEnvVars parses env vars from form text area and preserves secret refs.
func (v *CloudRunEditView) buildEnvVars(data map[string]any) map[string]string {
	envText, ok := data["env_vars"].(string)
	if !ok {
		return make(map[string]string)
	}
	envVars := make(map[string]string)

	// Parse KEY=value lines from text area
	for _, line := range strings.Split(envText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if k != "" {
				envVars[k] = val
			}
		}
	}

	// Note: Secret refs are NOT included in the update.
	// The API preserves secrets that aren't in the env list, so we don't need
	// to re-send them. Plain-text env vars are fully replaced.

	return envVars
}

// Task registration helpers
func (v *CloudRunEditView) registerTask(id, description string) {
	if v.ctx != nil {
		v.ctx.Tasks[id] = context.Task{
			ID:          id,
			Description: description,
			State:       context.TaskRunning,
			StartTime:   time.Now(),
		}
	}
}

func (v *CloudRunEditView) clearTask(id string) {
	if v.ctx != nil {
		delete(v.ctx.Tasks, id)
	}
}

// extractRegionFromFullName extracts the region from a Cloud Run full resource name.
func extractRegionFromFullName(fullName string) string {
	parts := strings.Split(fullName, "/")
	for i, part := range parts {
		if part == "locations" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// cloudRunRegions returns common Cloud Run regions.
func cloudRunRegions() []string {
	return []string{
		"us-central1",
		"us-east1",
		"us-east4",
		"us-west1",
		"us-west2",
		"us-west3",
		"us-west4",
		"us-south1",
		"northamerica-northeast1",
		"northamerica-northeast2",
		"southamerica-east1",
		"europe-west1",
		"europe-west2",
		"europe-west3",
		"europe-west4",
		"europe-west6",
		"europe-west8",
		"europe-west9",
		"europe-north1",
		"europe-central2",
		"asia-east1",
		"asia-east2",
		"asia-northeast1",
		"asia-northeast2",
		"asia-northeast3",
		"asia-south1",
		"asia-south2",
		"asia-southeast1",
		"asia-southeast2",
		"australia-southeast1",
		"australia-southeast2",
		"me-west1",
		"me-central1",
		"africa-south1",
	}
}
