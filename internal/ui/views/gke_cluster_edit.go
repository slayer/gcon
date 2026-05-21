package views

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/forms"
	"github.com/slayer/gcon/internal/ui/components/labeledit"
	"github.com/slayer/gcon/internal/ui/context"
)

// clusterEditState represents the view lifecycle.
type clusterEditState int

const (
	clusterEditStateForm          clusterEditState = iota
	clusterEditStateEditingLabels                  // labeledit overlay
	clusterEditStateDiff                           // preview old vs new
	clusterEditStateSaving                         // calling API (spinner)
)

// errClusterEditNoChanges is returned when the user submits without editing anything.
var errClusterEditNoChanges = errors.New("no changes to apply")

// sentinel errors for maintenance time validation.
var (
	errMaintenanceTimeFmt     = errors.New("time must be in HH:MM format (24-hour UTC)")
	errMaintenanceHourRange   = errors.New("hour must be 00–23")
	errMaintenanceMinuteRange = errors.New("minutes must be 00–59")
	errMaintenanceDailyNoTime = errors.New("daily maintenance window requires a start time")
	errMaintenanceDaysEmpty   = errors.New("recurring maintenance requires at least one day")
)

// clusterEditKeyMap defines key bindings for the cluster edit view.
type clusterEditKeyMap struct {
	Submit key.Binding
	Cancel key.Binding
	Enter  key.Binding
}

func defaultClusterEditKeyMap() clusterEditKeyMap {
	return clusterEditKeyMap{
		Submit: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "preview changes"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel / back to form"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm deploy"),
		),
	}
}

// GKEClusterEditView is a 4-state form view for editing a GKE cluster.
// States: form → editingLabels → diff (preview) → saving.
type GKEClusterEditView struct {
	projectID, location, clusterName string
	details                          *gcp.ClusterDetails
	Form                             *forms.Form

	state   clusterEditState
	err     error
	width   int
	height  int
	ctx     *context.ProgramContext
	spinner spinner.Model
	keys    clusterEditKeyMap

	// pendingBasic and pendingMaintenance are captured at submit time,
	// shown in the diff state, and sent on confirm-deploy.
	pendingBasic       *gcp.ClusterEdit
	pendingMaintenance *gcp.MaintenanceWindow

	// Labels editor sub-state.
	labelEditor   *labeledit.Editor
	editingLabels map[string]string // last-saved snapshot; nil = user never opened it
}

// NewGKEClusterEditView creates a new cluster edit view pre-populated from details.
func NewGKEClusterEditView(projectID, location, clusterName string, details *gcp.ClusterDetails) *GKEClusterEditView {
	v := &GKEClusterEditView{
		projectID:   projectID,
		location:    location,
		clusterName: clusterName,
		details:     details,
		spinner:     components.NewGCPSpinner(),
		keys:        defaultClusterEditKeyMap(),
	}
	v.buildForm()
	return v
}

// buildForm constructs the edit form with Basic, Maintenance, and Observability sections.
func (v *GKEClusterEditView) buildForm() {
	v.Form = forms.NewForm(
		fmt.Sprintf("Edit Cluster: %s", v.clusterName),
		forms.FormModeEdit,
	).EnableViewport()

	// ── Basic section ────────────────────────────────────────────────────────
	// Resource labels: show a read-only summary; user presses Enter or `l` to
	// open the labeledit overlay.
	labelsDisplay := labelsToDisplay(v.details.ResourceLabels)
	basicSection := forms.NewSection("basic", "Basic").
		AddField(forms.NewReadOnlyField("resource_labels", "Resource Labels", labelsDisplay).
			SetHelpText("Press Enter or `l` to edit"))

	v.Form.AddSection(basicSection)

	// ── Maintenance section ───────────────────────────────────────────────────
	initialKind := "none"
	switch {
	case v.details.MaintenanceDaily != "":
		initialKind = "daily"
	case v.details.MaintenanceRecurring != nil:
		initialKind = "recurring"
	}

	maintenanceSection := forms.NewSection("maintenance", "Maintenance").
		AddField(forms.NewDropdownField("maintenance_kind", "Maintenance Window").
			SetOptions([]forms.Option{
				{Value: "none", Label: "None (clear)"},
				{Value: "daily", Label: "Daily window"},
				{Value: "recurring", Label: "Recurring (weekly)"},
			}).
			SetHelpText("When to allow GKE to perform maintenance on the cluster")).
		AddField(forms.NewTextField("maintenance_daily_start", "Daily Start Time (UTC)").
			SetPlaceholder("HH:MM (UTC)").
			SetHelpText("Time the daily maintenance window begins, e.g. 03:00").
			SetValidator(validateMaintenanceTime)).
		AddField(forms.NewMultiSelectField("maintenance_days", "Days").
			SetOptions([]forms.Option{
				{Value: "MO", Label: "Mon"},
				{Value: "TU", Label: "Tue"},
				{Value: "WE", Label: "Wed"},
				{Value: "TH", Label: "Thu"},
				{Value: "FR", Label: "Fri"},
				{Value: "SA", Label: "Sat"},
				{Value: "SU", Label: "Sun"},
			}).
			SetHelpText("Days of week the recurring window applies")).
		AddField(forms.NewTextField("maintenance_recurring_start", "Recurring Start (UTC)").
			SetPlaceholder("HH:MM").
			SetHelpText("Time the recurring window begins each occurrence").
			SetValidator(validateMaintenanceTime)).
		AddField(forms.NewNumberField("maintenance_recurring_duration", "Recurring Duration (hours)").
			SetHelpText("How long each occurrence lasts (1-23)").
			SetValidator(forms.ValidateNumber(1, 23)))

	v.Form.AddSection(maintenanceSection)

	// ── Observability section ─────────────────────────────────────────────────
	loggingOptions := []forms.Option{
		{Value: "none", Label: "Disabled"},
		{Value: "logging.googleapis.com/kubernetes", Label: "System and workload"},
	}
	monitoringOptions := []forms.Option{
		{Value: "none", Label: "Disabled"},
		{Value: "monitoring.googleapis.com/kubernetes", Label: "System and workload"},
	}

	loggingField := forms.NewDropdownField("logging_service", "Logging").
		SetOptions(loggingOptions).
		SetHelpText("Cloud Logging integration for the cluster")
	monitoringField := forms.NewDropdownField("monitoring_service", "Monitoring").
		SetOptions(monitoringOptions).
		SetHelpText("Cloud Monitoring integration for the cluster")

	// If the current value is not in the curated list, show it as a placeholder.
	if !isKnownLoggingValue(v.details.LoggingService) && v.details.LoggingService != "" {
		loggingField.SetPlaceholder(v.details.LoggingService).
			SetHelpText(fmt.Sprintf("Current value %q is not in the curated list; select an option to change it", v.details.LoggingService))
	}
	if !isKnownMonitoringValue(v.details.MonitoringService) && v.details.MonitoringService != "" {
		monitoringField.SetPlaceholder(v.details.MonitoringService).
			SetHelpText(fmt.Sprintf("Current value %q is not in the curated list; select an option to change it", v.details.MonitoringService))
	}

	observabilitySection := forms.NewSection("observability", "Observability").
		AddField(loggingField).
		AddField(monitoringField)

	v.Form.AddSection(observabilitySection)

	// Pre-populate defaults from details.
	recurringDays := []string{"MO", "WE", "FR"}
	recurringStart := "03:00"
	var recurringDuration int64 = 4
	if v.details.MaintenanceRecurring != nil {
		rw := v.details.MaintenanceRecurring
		recurringDays = append([]string(nil), rw.Days...)
		recurringStart = rw.Start
		if h := parseDurationHours(rw.Duration); h > 0 {
			recurringDuration = int64(h)
		}
	}
	v.Form.SetData(map[string]any{
		"maintenance_kind":               initialKind,
		"maintenance_daily_start":        v.details.MaintenanceDaily,
		"maintenance_days":               recurringDays,
		"maintenance_recurring_start":    recurringStart,
		"maintenance_recurring_duration": recurringDuration,
	})

	// Sync visibility so initial kind hides the inapplicable fields.
	v.syncMaintenanceVisibility()

	// Pre-populate dropdowns only when the value is in the curated list.
	if isKnownLoggingValue(v.details.LoggingService) {
		v.Form.SetData(map[string]any{"logging_service": v.details.LoggingService})
	}
	if isKnownMonitoringValue(v.details.MonitoringService) {
		v.Form.SetData(map[string]any{"monitoring_service": v.details.MonitoringService})
	}
}

// validateMaintenanceTime accepts empty strings (meaning "no time set") or
// a valid HH:MM value.
func validateMaintenanceTime(value any) error {
	s, ok := value.(string)
	if !ok {
		return nil
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil // empty is allowed; the kind field controls whether it's required
	}
	// Validate HH:MM format: must be exactly 5 chars, colon at position 2,
	// all other characters must be ASCII digits.
	if len(s) != 5 || s[2] != ':' {
		return errMaintenanceTimeFmt
	}
	for _, i := range []int{0, 1, 3, 4} {
		if s[i] < '0' || s[i] > '9' {
			return errMaintenanceTimeFmt
		}
	}
	if s[0] != '0' && s[0] != '1' && s[0] != '2' {
		return errMaintenanceHourRange
	}
	hours := int(s[0]-'0')*10 + int(s[1]-'0')
	minutes := int(s[3]-'0')*10 + int(s[4]-'0')
	if hours > 23 {
		return errMaintenanceHourRange
	}
	if minutes > 59 {
		return errMaintenanceMinuteRange
	}
	return nil
}

// syncMaintenanceVisibility hides the maintenance fields that don't apply
// to the currently-selected kind. Called from buildForm (after SetData)
// and from Update on each key event (cheap, idempotent).
func (v *GKEClusterEditView) syncMaintenanceVisibility() {
	if v.Form == nil {
		return
	}
	var kind string
	if s, ok := v.Form.GetData()["maintenance_kind"].(string); ok {
		kind = s
	}
	if f := v.Form.GetField("maintenance_daily_start"); f != nil {
		f.SetHidden(kind != "daily")
	}
	for _, id := range []string{"maintenance_days", "maintenance_recurring_start", "maintenance_recurring_duration"} {
		if f := v.Form.GetField(id); f != nil {
			f.SetHidden(kind != "recurring")
		}
	}
}

// parseDurationHours parses a duration string like "4h" into an integer hour count.
// Returns 0 for values outside [1, 23] or unparseable strings.
func parseDurationHours(s string) int {
	s = strings.TrimSuffix(s, "h")
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 23 {
		return 0
	}
	return n
}

// isKnownLoggingValue returns true when the value is one of the dropdown options.
func isKnownLoggingValue(v string) bool {
	return v == "none" || v == "logging.googleapis.com/kubernetes"
}

// isKnownMonitoringValue returns true when the value is one of the dropdown options.
func isKnownMonitoringValue(v string) bool {
	return v == "none" || v == "monitoring.googleapis.com/kubernetes"
}

// labelsToDisplay formats a label map as "key=value, key=value" sorted alphabetically.
// Returns "(none)" when empty.
func labelsToDisplay(labels map[string]string) string {
	if len(labels) == 0 {
		return "(none)"
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, ", ")
}

// Init initializes the view.
func (v *GKEClusterEditView) Init() tea.Cmd {
	return tea.Batch(v.spinner.Tick, v.Form.Init())
}

// SetSize implements the View interface.
func (v *GKEClusterEditView) SetSize(width, height int) {
	v.width = width
	v.height = height
	if v.Form != nil {
		v.Form.SetSize(width-formWidthPadding, height-formHeightPadding)
	}
	if v.labelEditor != nil {
		v.labelEditor.SetSize(width-4, height-8)
	}
}

// SetContext implements the View interface.
func (v *GKEClusterEditView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.SetSize(ctx.ContentWidth, ctx.ContentHeight)
}

// HasTextInputFocused returns true when a text input is active.
func (v *GKEClusterEditView) HasTextInputFocused() bool {
	if v.state == clusterEditStateEditingLabels && v.labelEditor != nil {
		return v.labelEditor.HasTextInputFocused()
	}
	if v.state == clusterEditStateForm && v.Form != nil {
		return v.Form.HasTextInputFocused()
	}
	return false
}

// SetError resets the view to form state and displays the error.
func (v *GKEClusterEditView) SetError(err error) {
	v.state = clusterEditStateForm
	v.err = err
}

// openLabelEditor initializes the labeledit overlay and switches to editing state.
func (v *GKEClusterEditView) openLabelEditor() tea.Cmd {
	initial := v.editingLabels
	if initial == nil {
		// First open — clone from details so the editor has the current map.
		initial = make(map[string]string, len(v.details.ResourceLabels))
		for k, val := range v.details.ResourceLabels {
			initial[k] = val
		}
	}
	v.labelEditor = labeledit.New(initial)
	v.labelEditor.SetSize(v.width-4, v.height-8)
	v.state = clusterEditStateEditingLabels
	// labeledit.Editor has no Init method — return nil.
	return nil
}

// refreshLabelsDisplay updates the read-only labels field to reflect the
// most-recent editingLabels snapshot (or the original details if not yet opened).
func (v *GKEClusterEditView) refreshLabelsDisplay() {
	if v.Form == nil {
		return
	}
	f := v.Form.GetField("resource_labels")
	if f == nil {
		return
	}
	var current map[string]string
	if v.editingLabels != nil {
		current = v.editingLabels
	} else {
		current = v.details.ResourceLabels
	}
	f.SetValue(labelsToDisplay(current))
}

// Update handles messages for the cluster edit view.
func (v *GKEClusterEditView) Update(msg tea.Msg) tea.Cmd {
	// Route all messages to the labels editor while it is active.
	if v.state == clusterEditStateEditingLabels && v.labelEditor != nil {
		switch m := msg.(type) {
		case labeledit.SaveRequestedMsg:
			v.editingLabels = v.labelEditor.GetLabels()
			v.labelEditor = nil
			v.state = clusterEditStateForm
			v.refreshLabelsDisplay()
			return nil
		case tea.KeyMsg:
			// Esc when not in editing mode closes the editor without saving.
			if m.String() == "esc" && !v.labelEditor.IsEditing() {
				v.labelEditor = nil
				v.state = clusterEditStateForm
				return nil
			}
		}
		return v.labelEditor.Update(msg)
	}

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if v.state == clusterEditStateSaving {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case forms.FormSubmitMsg:
		return v.handleSubmit()

	case forms.FormCancelMsg:
		return func() tea.Msg { return GKEClusterEditCanceledMsg{} }

	case tea.KeyMsg:
		return v.handleKeyMsg(msg)
	}

	// Non-key, non-form-lifecycle messages (textinput.Blink, etc.) must reach
	// the form so the cursor blinks and any text-input commands run.
	if v.state == clusterEditStateForm && v.Form != nil {
		cmd := v.Form.Update(msg)
		v.syncMaintenanceVisibility()
		return cmd
	}
	return nil
}

func (v *GKEClusterEditView) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	// Saving state: only allow cancel.
	if v.state == clusterEditStateSaving {
		if key.Matches(msg, v.keys.Cancel) {
			return func() tea.Msg { return GKEClusterEditCanceledMsg{} }
		}
		return nil
	}

	// Diff state: Enter confirms deploy, Esc returns to form.
	if v.state == clusterEditStateDiff {
		if key.Matches(msg, v.keys.Enter) {
			return v.confirmDeploy()
		}
		if key.Matches(msg, v.keys.Cancel) {
			v.state = clusterEditStateForm
			return nil
		}
		return nil
	}

	// Form state.
	if v.state == clusterEditStateForm {
		if key.Matches(msg, v.keys.Submit) {
			return v.handleSubmit()
		}
		if key.Matches(msg, v.keys.Cancel) {
			return func() tea.Msg { return GKEClusterEditCanceledMsg{} }
		}
		// `l` opens the label editor when no text input is active (so typing
		// 'l' in a text field such as the maintenance time still works).
		if msg.String() == "l" && v.Form != nil && !v.Form.HasTextInputFocused() {
			return v.openLabelEditor()
		}
		if v.Form != nil {
			cmd := v.Form.Update(msg)
			v.syncMaintenanceVisibility()
			return cmd
		}
	}

	return nil
}

// handleSubmit validates the form, computes the edit diff, and transitions to the diff state.
func (v *GKEClusterEditView) handleSubmit() tea.Cmd {
	// Sync visibility before validating/extracting data so that the correct
	// fields are shown/hidden based on the currently selected maintenance kind.
	v.syncMaintenanceVisibility()
	if errs := v.Form.Validate(); len(errs) > 0 {
		return nil
	}
	data := v.Form.GetData()
	basic, maintenance, err := v.computeEdit(data)
	if err != nil {
		v.err = err
		return nil
	}
	if basic == nil && maintenance == nil {
		v.err = errClusterEditNoChanges
		return nil
	}
	v.pendingBasic = basic
	v.pendingMaintenance = maintenance
	v.err = nil
	v.state = clusterEditStateDiff
	return nil
}

// computeEdit compares form data against the snapshot and returns non-nil edit
// structs only for the categories that actually changed.
func (v *GKEClusterEditView) computeEdit(data map[string]any) (*gcp.ClusterEdit, *gcp.MaintenanceWindow, error) {
	getString := func(key string) string {
		if s, ok := data[key].(string); ok {
			return s
		}
		return ""
	}
	getInt64 := func(key string) int64 {
		if n, ok := data[key].(int64); ok {
			return n
		}
		return 0
	}
	getStringSlice := func(key string) []string {
		if v, ok := data[key].([]string); ok {
			return v
		}
		return nil
	}

	// ── Observability (logging / monitoring) ──────────────────────────────────
	// Baseline tracks what the form would return for "untouched". When the
	// cluster's current value is not in the curated dropdown list, the form
	// is left unpopulated and GetData returns the dropdown's default
	// (index 0 = "none"). Comparing the cluster's real unknown value would
	// flag a false-positive diff on every submit; instead we compare
	// against the same default so unchanged form == no diff.
	initialLogging := v.details.LoggingService
	if !isKnownLoggingValue(initialLogging) {
		initialLogging = "none"
	}
	initialMonitoring := v.details.MonitoringService
	if !isKnownMonitoringValue(initialMonitoring) {
		initialMonitoring = "none"
	}

	newLogging := getString("logging_service")
	newMonitoring := getString("monitoring_service")

	var basic *gcp.ClusterEdit
	if newLogging != initialLogging || newMonitoring != initialMonitoring {
		basic = &gcp.ClusterEdit{}
		if newLogging != initialLogging {
			s := newLogging
			basic.LoggingService = &s
		}
		if newMonitoring != initialMonitoring {
			s := newMonitoring
			basic.MonitoringService = &s
		}
	}

	// ── Maintenance window ────────────────────────────────────────────────────
	initialKind := gcp.MaintenanceKindNone
	switch {
	case v.details.MaintenanceDaily != "":
		initialKind = gcp.MaintenanceKindDaily
	case v.details.MaintenanceRecurring != nil:
		initialKind = gcp.MaintenanceKindRecurring
	}
	initialDaily := v.details.MaintenanceDaily

	newKindStr := getString("maintenance_kind")
	newKind := gcp.MaintenanceKind(newKindStr)

	var maint *gcp.MaintenanceWindow
	switch newKind {
	case gcp.MaintenanceKindNone:
		if initialKind != gcp.MaintenanceKindNone {
			maint = &gcp.MaintenanceWindow{Kind: gcp.MaintenanceKindNone}
		}

	case gcp.MaintenanceKindDaily:
		newDaily := strings.TrimSpace(getString("maintenance_daily_start"))
		if newDaily == "" {
			return nil, nil, errMaintenanceDailyNoTime
		}
		if initialKind != gcp.MaintenanceKindDaily || initialDaily != newDaily {
			maint = &gcp.MaintenanceWindow{Kind: gcp.MaintenanceKindDaily, Daily: newDaily}
		}

	case gcp.MaintenanceKindRecurring:
		days := getStringSlice("maintenance_days")
		start := strings.TrimSpace(getString("maintenance_recurring_start"))
		durationHours := getInt64("maintenance_recurring_duration")
		duration := fmt.Sprintf("%dh", durationHours)

		if len(days) == 0 {
			return nil, nil, errMaintenanceDaysEmpty
		}

		// Baseline comparison: same kind + same days (sorted) + same start + same duration → no change.
		var initialDays []string
		initialStart := ""
		initialDuration := ""
		if v.details.MaintenanceRecurring != nil {
			initialDays = append([]string(nil), v.details.MaintenanceRecurring.Days...)
			initialStart = v.details.MaintenanceRecurring.Start
			initialDuration = v.details.MaintenanceRecurring.Duration
		}

		sortedNew := append([]string(nil), days...)
		sort.Strings(sortedNew)
		sortedInit := append([]string(nil), initialDays...)
		sort.Strings(sortedInit)

		if initialKind != gcp.MaintenanceKindRecurring ||
			!slicesEqual(sortedInit, sortedNew) ||
			initialStart != start ||
			initialDuration != duration {
			maint = &gcp.MaintenanceWindow{
				Kind:     gcp.MaintenanceKindRecurring,
				Days:     sortedNew,
				Start:    start,
				Duration: duration,
			}
		}
	}

	// ── Resource labels ───────────────────────────────────────────────────────
	if v.editingLabels != nil && !mapsEqual(v.details.ResourceLabels, v.editingLabels) {
		if basic == nil {
			basic = &gcp.ClusterEdit{}
		}
		cloned := make(map[string]string, len(v.editingLabels))
		for k, val := range v.editingLabels {
			cloned[k] = val
		}
		basic.ResourceLabels = &cloned
		basic.ResourceLabelsFingerprint = v.details.ResourceLabelsFingerprint
	}

	return basic, maint, nil
}

// confirmDeploy transitions to saving state and emits the edit request.
func (v *GKEClusterEditView) confirmDeploy() tea.Cmd {
	v.state = clusterEditStateSaving
	basic := v.pendingBasic
	maintenance := v.pendingMaintenance
	return tea.Batch(
		v.spinner.Tick,
		func() tea.Msg {
			return GKEClusterEditRequestMsg{
				ProjectID:   v.projectID,
				Location:    v.location,
				ClusterName: v.clusterName,
				Basic:       basic,
				Maintenance: maintenance,
			}
		},
	)
}

// View renders the current state.
func (v *GKEClusterEditView) View() string {
	switch v.state {
	case clusterEditStateEditingLabels:
		if v.labelEditor != nil {
			return v.labelEditor.View()
		}

	case clusterEditStateSaving:
		return renderSaving(v.spinner, "Updating cluster...")

	case clusterEditStateDiff:
		return v.renderDiff()

	case clusterEditStateForm:
		content := ""
		if v.Form != nil {
			content = v.Form.View()
		}
		if v.err != nil {
			content += components.RenderInlineError(v.err)
		}
		return content
	}

	return ""
}

// renderDiff renders a color-coded preview of pending changes.
func (v *GKEClusterEditView) renderDiff() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9AA0A6"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Faint(true)

	b.WriteString("\n")
	b.WriteString(titleStyle.Render(fmt.Sprintf("  Review changes for cluster %q:", v.clusterName)))
	b.WriteString("\n\n")

	anyChange := false

	// ── Resource labels ───────────────────────────────────────────────────────
	if v.pendingBasic != nil && v.pendingBasic.ResourceLabels != nil {
		b.WriteString(sectionStyle.Render("  Basic:"))
		b.WriteString("\n")

		addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
		removedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		changedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))

		newLabels := *v.pendingBasic.ResourceLabels
		oldLabels := v.details.ResourceLabels

		keys := mergedSortedKeys(oldLabels, newLabels)
		for _, k := range keys {
			oldV, hadOld := oldLabels[k]
			newV, hasNew := newLabels[k]
			switch {
			case hadOld && !hasNew:
				b.WriteString(removedStyle.Render(fmt.Sprintf("    - %s=%s", k, oldV)))
			case !hadOld && hasNew:
				b.WriteString(addedStyle.Render(fmt.Sprintf("    + %s=%s", k, newV)))
			case hadOld && hasNew && oldV != newV:
				b.WriteString(changedStyle.Render(fmt.Sprintf("    ! %s: %s → %s", k, oldV, newV)))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		anyChange = true
	}

	// ── Observability ─────────────────────────────────────────────────────────
	if v.pendingBasic != nil && (v.pendingBasic.LoggingService != nil || v.pendingBasic.MonitoringService != nil) {
		b.WriteString(sectionStyle.Render("  Observability:"))
		b.WriteString("\n")
		if v.pendingBasic.LoggingService != nil {
			old := v.details.LoggingService
			nw := *v.pendingBasic.LoggingService
			line := fmt.Sprintf("    %-28s %s → %s", "logging_service:", old, nw)
			b.WriteString(diffServiceStyle(old, nw).Render(line))
			b.WriteString("\n")
			anyChange = true
		}
		if v.pendingBasic.MonitoringService != nil {
			old := v.details.MonitoringService
			nw := *v.pendingBasic.MonitoringService
			line := fmt.Sprintf("    %-28s %s → %s", "monitoring_service:", old, nw)
			b.WriteString(diffServiceStyle(old, nw).Render(line))
			b.WriteString("\n")
			anyChange = true
		}
		b.WriteString("\n")
	}

	// ── Maintenance ───────────────────────────────────────────────────────────
	if v.pendingMaintenance != nil {
		b.WriteString(sectionStyle.Render("  Maintenance:"))
		b.WriteString("\n")
		changed := v.renderMaintenanceDiff()
		b.WriteString(changed)
		b.WriteString("\n")
		anyChange = true
	}

	if !anyChange {
		b.WriteString(mutedStyle.Render("  (no changes)"))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("  Enter: apply   Esc: back to form"))
	b.WriteString("\n")

	return b.String()
}

// renderMaintenanceDiff renders the per-field maintenance diff lines.
// Called from renderDiff when pendingMaintenance is non-nil.
func (v *GKEClusterEditView) renderMaintenanceDiff() string {
	var b strings.Builder

	changedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))
	addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))

	initialKind := gcp.MaintenanceKindNone
	switch {
	case v.details.MaintenanceDaily != "":
		initialKind = gcp.MaintenanceKindDaily
	case v.details.MaintenanceRecurring != nil:
		initialKind = gcp.MaintenanceKindRecurring
	}

	if v.pendingMaintenance.Kind != initialKind {
		line := fmt.Sprintf("    %-28s %s → %s", "kind:", string(initialKind), string(v.pendingMaintenance.Kind))
		b.WriteString(changedStyle.Render(line))
		b.WriteString("\n")
	}

	switch v.pendingMaintenance.Kind {
	case gcp.MaintenanceKindDaily:
		old := v.details.MaintenanceDaily
		if old == "" {
			old = "(none)"
		}
		nw := v.pendingMaintenance.Daily
		if nw == "" {
			nw = "(none)"
		}
		if old != nw {
			line := fmt.Sprintf("    %-28s %s → %s", "daily_start:", old, nw)
			b.WriteString(diffMaintenanceDailyStyle(v.pendingMaintenance.Kind, v.details.MaintenanceDaily).Render(line))
			b.WriteString("\n")
		}

	case gcp.MaintenanceKindRecurring:
		b.WriteString(v.renderRecurringDiff(changedStyle, addedStyle))
	}

	return b.String()
}

// renderRecurringDiff renders the days/start/duration delta lines for a recurring window change.
func (v *GKEClusterEditView) renderRecurringDiff(changedStyle, addedStyle lipgloss.Style) string {
	var b strings.Builder

	var oldDays []string
	oldStart := ""
	oldDuration := ""
	if v.details.MaintenanceRecurring != nil {
		oldDays = v.details.MaintenanceRecurring.Days
		oldStart = v.details.MaintenanceRecurring.Start
		oldDuration = v.details.MaintenanceRecurring.Duration
	}

	newDaysStr := strings.Join(v.pendingMaintenance.Days, ",")
	oldDaysStr := strings.Join(oldDays, ",")
	if oldDaysStr != newDaysStr {
		if oldDaysStr == "" {
			oldDaysStr = "(none)"
		}
		line := fmt.Sprintf("    %-28s %s → %s", "days:", oldDaysStr, newDaysStr)
		b.WriteString(addedStyle.Render(line))
		b.WriteString("\n")
	}
	if oldStart != v.pendingMaintenance.Start {
		old := oldStart
		if old == "" {
			old = "(none)"
		}
		line := fmt.Sprintf("    %-28s %s → %s", "start:", old, v.pendingMaintenance.Start)
		b.WriteString(changedStyle.Render(line))
		b.WriteString("\n")
	}
	if oldDuration != v.pendingMaintenance.Duration {
		old := oldDuration
		if old == "" {
			old = "(none)"
		}
		line := fmt.Sprintf("    %-28s %s → %s", "duration:", old, v.pendingMaintenance.Duration)
		b.WriteString(changedStyle.Render(line))
		b.WriteString("\n")
	}

	return b.String()
}

// diffServiceStyle returns the diff color for a service (logging/monitoring) change.
// Green = new value being added (old was empty); red = being disabled; yellow = modified.
func diffServiceStyle(old, newVal string) lipgloss.Style {
	switch {
	case old == "":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	case newVal == "" || newVal == "none":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))
	}
}

// diffMaintenanceDailyStyle returns the diff color for a daily_start change.
// Red = window being cleared; green = new window added; yellow = time changed.
func diffMaintenanceDailyStyle(newKind gcp.MaintenanceKind, oldDaily string) lipgloss.Style {
	switch {
	case newKind == gcp.MaintenanceKindNone:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	case oldDaily == "":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))
	}
}

// mapsEqual returns true when a and b contain exactly the same key-value pairs.
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// mergedSortedKeys returns a deduplicated, alphabetically sorted slice of all
// keys present in either a or b.
func mergedSortedKeys(a, b map[string]string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
