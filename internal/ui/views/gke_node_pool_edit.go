package views

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/forms"
	"github.com/slayer/gcon/internal/ui/components/labeledit"
	"github.com/slayer/gcon/internal/ui/components/taintedit"
	"github.com/slayer/gcon/internal/ui/context"
)

// nodePoolEditState represents the view lifecycle.
type nodePoolEditState int

const (
	nodePoolEditStateForm          nodePoolEditState = iota
	nodePoolEditStateEditingLabels                   // labeledit overlay
	nodePoolEditStateEditingTaints                   // taintedit overlay (Task 8)
	nodePoolEditStateDiff                            // preview old vs new
	nodePoolEditStateSaving                          // calling API (spinner)
)

// k8s label key: optional DNS prefix (subdomain/name) + name segment.
var k8sLabelKeyPattern = regexp.MustCompile(
	`^([a-z0-9]([-a-z0-9.]*[a-z0-9])?/)?[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`,
)

// k8s label value: empty OR alphanumeric start/end with hyphens/underscores/dots inside.
var k8sLabelValuePattern = regexp.MustCompile(
	`^$|^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`,
)

// errNodePoolEditNoChanges is returned when the user submits without editing anything.
var errNodePoolEditNoChanges = errors.New("no changes to apply")

// errNodePoolEditSurgeUnavailable is returned when max_surge + max_unavailable < 1.
var errNodePoolEditSurgeUnavailable = errors.New("max_surge + max_unavailable must be >= 1")

// nodePoolEditKeyMap defines key bindings for the node pool edit view.
type nodePoolEditKeyMap struct {
	Submit key.Binding
	Cancel key.Binding
	Enter  key.Binding
}

func defaultNodePoolEditKeyMap() nodePoolEditKeyMap {
	return nodePoolEditKeyMap{
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

// GKENodePoolEditView is a multi-state form view for editing a GKE node pool.
// States: form → editingLabels → editingTaints → diff (preview) → saving.
type GKENodePoolEditView struct {
	projectID, location, clusterName, poolName string
	pool                                       *gcp.NodePool
	Form                                       *forms.Form

	state   nodePoolEditState
	err     error
	width   int
	height  int
	ctx     *context.ProgramContext
	spinner spinner.Model
	keys    nodePoolEditKeyMap

	// pendingFields and pendingManagement are captured at submit time,
	// shown in the diff state, and sent on confirm-deploy.
	pendingFields     *gcp.NodePoolEdit
	pendingManagement *gcp.NodePoolManagement

	// Labels editor sub-state (Task 7).
	labelEditor   *labeledit.Editor
	editingLabels map[string]string // last-saved snapshot; nil = user never opened it

	// Taints editor sub-state (Task 8).
	taintEditor   *taintedit.Editor
	editingTaints []gcp.NodeTaint // last-saved snapshot; nil = user never opened it
}

// NewGKENodePoolEditView creates a new node pool edit view pre-populated from pool.
func NewGKENodePoolEditView(projectID, location, clusterName string, pool *gcp.NodePool) *GKENodePoolEditView {
	v := &GKENodePoolEditView{
		projectID:   projectID,
		location:    location,
		clusterName: clusterName,
		poolName:    pool.Name,
		pool:        pool,
		spinner:     components.NewGCPSpinner(),
		keys:        defaultNodePoolEditKeyMap(),
	}
	v.buildForm()
	return v
}

// buildForm constructs the edit form with Labels, Taints, Management, and Upgrade Settings sections.
func (v *GKENodePoolEditView) buildForm() {
	v.Form = forms.NewForm(
		fmt.Sprintf("Edit Node Pool: %s", v.poolName),
		forms.FormModeEdit,
	).EnableViewport()

	// ── Labels section ────────────────────────────────────────────────────────
	// k8s labels on NodeConfig — press `l` to open the labeledit overlay.
	labelsDisplay := labelsToDisplay(v.pool.Labels)
	labelsSection := forms.NewSection("labels", "Node Labels").
		AddField(forms.NewReadOnlyField("node_labels", "Node Labels", labelsDisplay).
			SetHelpText("Press `l` to edit"))

	v.Form.AddSection(labelsSection)

	// ── Taints section ────────────────────────────────────────────────────────
	// Node taints — press `t` to open the taintedit overlay.
	taintsDisplay := taintsToDisplay(v.pool.Taints)
	taintsSection := forms.NewSection("taints", "Node Taints").
		AddField(forms.NewReadOnlyField("node_taints", "Node Taints", taintsDisplay).
			SetHelpText("Press `t` to edit"))

	v.Form.AddSection(taintsSection)

	// ── Management section ────────────────────────────────────────────────────
	managementSection := forms.NewSection("management", "Management").
		AddField(forms.NewToggleField("auto_upgrade", "Auto Upgrade").
			SetHelpText("Automatically upgrade nodes to the latest version")).
		AddField(forms.NewToggleField("auto_repair", "Auto Repair").
			SetHelpText("Automatically repair unhealthy nodes"))

	v.Form.AddSection(managementSection)

	// ── Upgrade Settings section ──────────────────────────────────────────────
	upgradeHelpText := ""
	initialStrategy := "SURGE"
	var initialMaxSurge int64 = 1
	var initialMaxUnavailable int64
	unknownStrategy := ""

	if v.pool.UpgradeSettings == nil {
		upgradeHelpText = "Pool has no explicit upgrade settings; change a value to set them."
	} else {
		switch v.pool.UpgradeSettings.Strategy {
		case "SURGE", "BLUE_GREEN":
			initialStrategy = v.pool.UpgradeSettings.Strategy
		case "":
			// empty == server default; treat as SURGE
		default:
			// Unknown strategy (e.g., legacy SHORT_LIVED). Don't pre-select it
			// into the curated dropdown — the form would silently fall back
			// to SURGE and computeEdit would then see a false-positive diff.
			// Force the baseline to SURGE so unchanged form == no diff; the
			// placeholder tells the user what the pool actually has now.
			unknownStrategy = v.pool.UpgradeSettings.Strategy
		}
		initialMaxSurge = v.pool.UpgradeSettings.MaxSurge
		initialMaxUnavailable = v.pool.UpgradeSettings.MaxUnavailable
	}

	strategyField := forms.NewDropdownField("upgrade_strategy", "Upgrade Strategy").
		SetOptions([]forms.Option{
			{Value: "SURGE", Label: "Surge"},
			{Value: "BLUE_GREEN", Label: "Blue-Green"},
		}).
		SetHelpText("Strategy for upgrading node pool nodes")
	if unknownStrategy != "" {
		strategyField.SetPlaceholder(unknownStrategy)
		strategyField.SetHelpText(fmt.Sprintf("Pool currently uses %q (not selectable here). Pick SURGE or BLUE_GREEN to migrate.", unknownStrategy))
	}

	maxSurgeField := forms.NewNumberField("max_surge", "Max Surge").
		SetHelpText("Max additional nodes during upgrade (0-100)").
		SetValidator(forms.ValidateNumber(0, 100))

	maxUnavailableField := forms.NewNumberField("max_unavailable", "Max Unavailable").
		SetHelpText("Max nodes unavailable during upgrade (0-100)").
		SetValidator(forms.ValidateNumber(0, 100))

	upgradeSection := forms.NewSection("upgrade_settings", "Upgrade Settings").
		AddField(strategyField).
		AddField(maxSurgeField).
		AddField(maxUnavailableField)

	if upgradeHelpText != "" {
		upgradeSection.SetDescription(upgradeHelpText)
	}

	v.Form.AddSection(upgradeSection)

	// Pre-populate defaults from pool.
	v.Form.SetData(map[string]any{
		"auto_upgrade":     v.pool.AutoUpgrade,
		"auto_repair":      v.pool.AutoRepair,
		"upgrade_strategy": initialStrategy,
		"max_surge":        initialMaxSurge,
		"max_unavailable":  initialMaxUnavailable,
	})
}

// taintsToDisplay formats a slice of node taints as "key=value:effect, ..." sorted by key.
// Returns "(none)" when empty.
func taintsToDisplay(taints []gcp.NodeTaint) string {
	if len(taints) == 0 {
		return "(none)"
	}
	sorted := make([]gcp.NodeTaint, len(taints))
	copy(sorted, taints)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Key < sorted[j].Key
	})
	parts := make([]string, 0, len(sorted))
	for _, t := range sorted {
		parts = append(parts, fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect))
	}
	return strings.Join(parts, ", ")
}

// openLabelEditor initializes the labeledit overlay with k8s validators and
// switches to the editing state.
func (v *GKENodePoolEditView) openLabelEditor() tea.Cmd {
	initial := v.editingLabels
	if initial == nil {
		// First open — clone from pool so the editor has the current map.
		initial = make(map[string]string, len(v.pool.Labels))
		for k, val := range v.pool.Labels {
			initial[k] = val
		}
	}
	v.labelEditor = labeledit.New(initial)
	v.labelEditor.SetValidators(labeledit.Validators{
		KeyPattern:   k8sLabelKeyPattern,
		ValuePattern: k8sLabelValuePattern,
		KeyError:     "Invalid k8s label key (optional DNS prefix + name)",
		ValueError:   "Invalid k8s label value (may be empty)",
	})
	v.labelEditor.SetSize(v.width-4, v.height-8)
	v.state = nodePoolEditStateEditingLabels
	// labeledit.Editor has no Init method — return nil.
	return nil
}

// refreshLabelsDisplay updates the read-only labels field to reflect the
// most-recent editingLabels snapshot (or the original pool if not yet opened).
func (v *GKENodePoolEditView) refreshLabelsDisplay() {
	if v.Form == nil {
		return
	}
	f := v.Form.GetField("node_labels")
	if f == nil {
		return
	}
	var current map[string]string
	if v.editingLabels != nil {
		current = v.editingLabels
	} else {
		current = v.pool.Labels
	}
	f.SetValue(labelsToDisplay(current))
}

// openTaintEditor initializes the taintedit overlay and switches to the editing state.
func (v *GKENodePoolEditView) openTaintEditor() tea.Cmd {
	initial := v.editingTaints
	if initial == nil {
		// First open — clone from pool.
		initial = append([]gcp.NodeTaint(nil), v.pool.Taints...)
	}
	v.taintEditor = taintedit.New(initial)
	v.taintEditor.SetSize(v.width-4, v.height-8)
	v.state = nodePoolEditStateEditingTaints
	return nil
}

// refreshTaintsDisplay updates the read-only taints field to reflect the
// most-recent editingTaints snapshot (or the original pool if not yet opened).
func (v *GKENodePoolEditView) refreshTaintsDisplay() {
	if v.Form == nil {
		return
	}
	f := v.Form.GetField("node_taints")
	if f == nil {
		return
	}
	var current []gcp.NodeTaint
	if v.editingTaints != nil {
		current = v.editingTaints
	} else {
		current = v.pool.Taints
	}
	f.SetValue(taintsToDisplay(current))
}

// Init initializes the view.
func (v *GKENodePoolEditView) Init() tea.Cmd {
	return tea.Batch(v.spinner.Tick, v.Form.Init())
}

// SetSize implements the View interface.
func (v *GKENodePoolEditView) SetSize(width, height int) {
	v.width = width
	v.height = height
	if v.Form != nil {
		v.Form.SetSize(width-formWidthPadding, height-formHeightPadding)
	}
	if v.labelEditor != nil {
		v.labelEditor.SetSize(width-4, height-8)
	}
	if v.taintEditor != nil {
		v.taintEditor.SetSize(width-4, height-8)
	}
}

// SetContext implements the View interface.
func (v *GKENodePoolEditView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.SetSize(ctx.ContentWidth, ctx.ContentHeight)
}

// HasTextInputFocused returns true when a text input is active.
func (v *GKENodePoolEditView) HasTextInputFocused() bool {
	if v.state == nodePoolEditStateEditingLabels && v.labelEditor != nil {
		return v.labelEditor.HasTextInputFocused()
	}
	if v.state == nodePoolEditStateEditingTaints && v.taintEditor != nil {
		return v.taintEditor.HasTextInputFocused()
	}
	if v.state == nodePoolEditStateForm && v.Form != nil {
		return v.Form.HasTextInputFocused()
	}
	return false
}

// SetError resets the view to form state and displays the error.
func (v *GKENodePoolEditView) SetError(err error) {
	v.state = nodePoolEditStateForm
	v.err = err
}

// Update handles messages for the node pool edit view.
func (v *GKENodePoolEditView) Update(msg tea.Msg) tea.Cmd {
	// Route all messages to the labels editor while it is active.
	if v.state == nodePoolEditStateEditingLabels && v.labelEditor != nil {
		switch m := msg.(type) {
		case labeledit.SaveRequestedMsg:
			v.editingLabels = v.labelEditor.GetLabels()
			v.labelEditor = nil
			v.state = nodePoolEditStateForm
			v.refreshLabelsDisplay()
			return nil
		case tea.KeyMsg:
			// Esc when not in row-edit mode closes the editor without saving.
			if m.String() == "esc" && !v.labelEditor.IsEditing() {
				v.labelEditor = nil
				v.state = nodePoolEditStateForm
				return nil
			}
		}
		return v.labelEditor.Update(msg)
	}

	// Route all messages to the taints editor while it is active.
	if v.state == nodePoolEditStateEditingTaints && v.taintEditor != nil {
		switch msg.(type) {
		case taintedit.SaveRequestedMsg:
			v.editingTaints = v.taintEditor.GetTaints()
			v.taintEditor = nil
			v.state = nodePoolEditStateForm
			v.refreshTaintsDisplay()
			return nil
		case taintedit.CancelRequestedMsg:
			v.taintEditor = nil
			v.state = nodePoolEditStateForm
			return nil
		}
		return v.taintEditor.Update(msg)
	}

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if v.state == nodePoolEditStateSaving {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case forms.FormSubmitMsg:
		return v.handleSubmit()

	case forms.FormCancelMsg:
		return func() tea.Msg { return GKENodePoolEditCanceledMsg{} }

	case tea.KeyMsg:
		return v.handleKeyMsg(msg)
	}

	// Non-key, non-form-lifecycle messages (textinput.Blink, etc.) must reach
	// the form so the cursor blinks and any text-input commands run.
	if v.state == nodePoolEditStateForm && v.Form != nil {
		return v.Form.Update(msg)
	}
	return nil
}

func (v *GKENodePoolEditView) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	// Saving state: only allow cancel.
	if v.state == nodePoolEditStateSaving {
		if key.Matches(msg, v.keys.Cancel) {
			return func() tea.Msg { return GKENodePoolEditCanceledMsg{} }
		}
		return nil
	}

	// Diff state: Enter confirms deploy, Esc returns to form.
	if v.state == nodePoolEditStateDiff {
		if key.Matches(msg, v.keys.Enter) {
			return v.confirmDeploy()
		}
		if key.Matches(msg, v.keys.Cancel) {
			v.state = nodePoolEditStateForm
			return nil
		}
		return nil
	}

	// Form state.
	if v.state == nodePoolEditStateForm {
		if key.Matches(msg, v.keys.Submit) {
			return v.handleSubmit()
		}
		if key.Matches(msg, v.keys.Cancel) {
			return func() tea.Msg { return GKENodePoolEditCanceledMsg{} }
		}
		// `l` opens the label editor when no text input is active.
		if msg.String() == "l" && v.Form != nil && !v.Form.HasTextInputFocused() {
			return v.openLabelEditor()
		}
		// `t` opens the taint editor when no text input is active.
		if msg.String() == "t" && v.Form != nil && !v.Form.HasTextInputFocused() {
			return v.openTaintEditor()
		}
		if v.Form != nil {
			return v.Form.Update(msg)
		}
	}

	return nil
}

// handleSubmit validates the form, computes the edit diff, and transitions to the diff state.
func (v *GKENodePoolEditView) handleSubmit() tea.Cmd {
	if errs := v.Form.Validate(); len(errs) > 0 {
		return nil
	}
	data := v.Form.GetData()
	fields, management, err := v.computeEdit(data)
	if err != nil {
		v.err = err
		return nil
	}
	if fields == nil && management == nil {
		v.err = errNodePoolEditNoChanges
		return nil
	}
	v.pendingFields = fields
	v.pendingManagement = management
	v.err = nil
	v.state = nodePoolEditStateDiff
	return nil
}

// computeEdit compares form data against the pool snapshot and returns non-nil edit
// structs only for the categories that actually changed.
func (v *GKENodePoolEditView) computeEdit(data map[string]any) (*gcp.NodePoolEdit, *gcp.NodePoolManagement, error) {
	getBool := func(k string) bool {
		if b, ok := data[k].(bool); ok {
			return b
		}
		return false
	}
	getString := func(k string) string {
		if s, ok := data[k].(string); ok {
			return s
		}
		return ""
	}
	getInt64 := func(k string) int64 {
		if n, ok := data[k].(int64); ok {
			return n
		}
		return 0
	}

	// ── Management ────────────────────────────────────────────────────────────
	newAutoUpgrade := getBool("auto_upgrade")
	newAutoRepair := getBool("auto_repair")

	var management *gcp.NodePoolManagement
	if newAutoUpgrade != v.pool.AutoUpgrade || newAutoRepair != v.pool.AutoRepair {
		management = &gcp.NodePoolManagement{
			AutoUpgrade: newAutoUpgrade,
			AutoRepair:  newAutoRepair,
		}
	}

	// ── Upgrade settings ──────────────────────────────────────────────────────
	initialStrategy, initialMaxSurge, initialMaxUnavailable := v.initialUpgradeSettings()

	newStrategy := getString("upgrade_strategy")
	newMaxSurge := getInt64("max_surge")
	newMaxUnavailable := getInt64("max_unavailable")

	upgradeChanged := newStrategy != initialStrategy ||
		newMaxSurge != initialMaxSurge ||
		newMaxUnavailable != initialMaxUnavailable

	if upgradeChanged {
		// Cross-validation: max_surge + max_unavailable must be >= 1.
		if newMaxSurge+newMaxUnavailable < 1 {
			return nil, nil, errNodePoolEditSurgeUnavailable
		}
	}

	var fields *gcp.NodePoolEdit
	if upgradeChanged {
		fields = &gcp.NodePoolEdit{
			UpgradeSettings: &gcp.UpgradeSettings{
				Strategy:       newStrategy,
				MaxSurge:       newMaxSurge,
				MaxUnavailable: newMaxUnavailable,
			},
		}
	}

	// ── Labels ────────────────────────────────────────────────────────────────
	if v.editingLabels != nil && !mapsEqual(v.pool.Labels, v.editingLabels) {
		if fields == nil {
			fields = &gcp.NodePoolEdit{}
		}
		cloned := make(map[string]string, len(v.editingLabels))
		for k, val := range v.editingLabels {
			cloned[k] = val
		}
		fields.Labels = &cloned
	}

	// ── Taints ────────────────────────────────────────────────────────────────
	if v.editingTaints != nil && !taintsEqual(v.pool.Taints, v.editingTaints) {
		if fields == nil {
			fields = &gcp.NodePoolEdit{}
		}
		cloned := append([]gcp.NodeTaint(nil), v.editingTaints...)
		fields.Taints = &cloned
	}

	return fields, management, nil
}

// taintsEqual returns true when a and b contain the same taints (order-insensitive).
func taintsEqual(a, b []gcp.NodeTaint) bool {
	if len(a) != len(b) {
		return false
	}
	sa := append([]gcp.NodeTaint(nil), a...)
	sb := append([]gcp.NodeTaint(nil), b...)
	sort.Slice(sa, func(i, j int) bool { return sa[i].Key < sa[j].Key })
	sort.Slice(sb, func(i, j int) bool { return sb[i].Key < sb[j].Key })
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}

// confirmDeploy transitions to saving state and emits the edit request.
func (v *GKENodePoolEditView) confirmDeploy() tea.Cmd {
	v.state = nodePoolEditStateSaving
	fields := v.pendingFields
	management := v.pendingManagement
	return tea.Batch(
		v.spinner.Tick,
		func() tea.Msg {
			return GKENodePoolEditRequestMsg{
				ProjectID:   v.projectID,
				Location:    v.location,
				ClusterName: v.clusterName,
				PoolName:    v.poolName,
				Fields:      fields,
				Management:  management,
			}
		},
	)
}

// View renders the current state.
func (v *GKENodePoolEditView) View() string {
	switch v.state {
	case nodePoolEditStateEditingLabels:
		if v.labelEditor != nil {
			return v.labelEditor.View()
		}

	case nodePoolEditStateEditingTaints:
		if v.taintEditor != nil {
			return v.taintEditor.View()
		}

	case nodePoolEditStateSaving:
		return renderSaving(v.spinner, "Updating node pool...")

	case nodePoolEditStateDiff:
		return v.renderDiff()

	case nodePoolEditStateForm:
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
func (v *GKENodePoolEditView) renderDiff() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9AA0A6"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Faint(true)
	changedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))

	b.WriteString("\n")
	b.WriteString(titleStyle.Render(fmt.Sprintf("  Review changes for node pool %q:", v.poolName)))
	b.WriteString("\n\n")

	anyChange := false

	// ── Labels ────────────────────────────────────────────────────────────────
	if v.pendingFields != nil && v.pendingFields.Labels != nil {
		b.WriteString(sectionStyle.Render("  Node Labels:"))
		b.WriteString("\n")

		addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
		removedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
		labelChangedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))

		newLabels := *v.pendingFields.Labels
		oldLabels := v.pool.Labels

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
				b.WriteString(labelChangedStyle.Render(fmt.Sprintf("    ! %s: %s → %s", k, oldV, newV)))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		anyChange = true
	}

	// ── Management ────────────────────────────────────────────────────────────
	if v.pendingManagement != nil {
		b.WriteString(sectionStyle.Render("  Management:"))
		b.WriteString("\n")
		if v.pendingManagement.AutoUpgrade != v.pool.AutoUpgrade {
			old := boolToStr(v.pool.AutoUpgrade)
			nw := boolToStr(v.pendingManagement.AutoUpgrade)
			line := fmt.Sprintf("    %-28s %s → %s", "auto_upgrade:", old, nw)
			b.WriteString(changedStyle.Render(line))
			b.WriteString("\n")
			anyChange = true
		}
		if v.pendingManagement.AutoRepair != v.pool.AutoRepair {
			old := boolToStr(v.pool.AutoRepair)
			nw := boolToStr(v.pendingManagement.AutoRepair)
			line := fmt.Sprintf("    %-28s %s → %s", "auto_repair:", old, nw)
			b.WriteString(changedStyle.Render(line))
			b.WriteString("\n")
			anyChange = true
		}
		b.WriteString("\n")
	}

	// ── Upgrade Settings ──────────────────────────────────────────────────────
	if v.pendingFields != nil && v.pendingFields.UpgradeSettings != nil {
		b.WriteString(sectionStyle.Render("  Upgrade Settings:"))
		b.WriteString("\n")

		initialStrategy, initialMaxSurge, initialMaxUnavailable := v.initialUpgradeSettings()

		us := v.pendingFields.UpgradeSettings
		if us.Strategy != initialStrategy {
			line := fmt.Sprintf("    %-28s %s → %s", "upgrade_strategy:", initialStrategy, us.Strategy)
			b.WriteString(changedStyle.Render(line))
			b.WriteString("\n")
			anyChange = true
		}
		if us.MaxSurge != initialMaxSurge {
			line := fmt.Sprintf("    %-28s %d → %d", "max_surge:", initialMaxSurge, us.MaxSurge)
			b.WriteString(changedStyle.Render(line))
			b.WriteString("\n")
			anyChange = true
		}
		if us.MaxUnavailable != initialMaxUnavailable {
			line := fmt.Sprintf("    %-28s %d → %d", "max_unavailable:", initialMaxUnavailable, us.MaxUnavailable)
			b.WriteString(changedStyle.Render(line))
			b.WriteString("\n")
			anyChange = true
		}
		b.WriteString("\n")
	}

	// ── Taints ────────────────────────────────────────────────────────────────
	if v.pendingFields != nil && v.pendingFields.Taints != nil {
		b.WriteString(sectionStyle.Render("  Node Taints:"))
		b.WriteString("\n")

		addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
		removedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))

		newTaints := *v.pendingFields.Taints
		oldTaints := v.pool.Taints

		// Build maps keyed by taint key for diff display.
		oldByKey := make(map[string]gcp.NodeTaint, len(oldTaints))
		for _, t := range oldTaints {
			oldByKey[t.Key] = t
		}
		newByKey := make(map[string]gcp.NodeTaint, len(newTaints))
		for _, t := range newTaints {
			newByKey[t.Key] = t
		}
		// Collect all keys from both sides, sorted.
		allKeys := mergedSortedKeys(
			func() map[string]string {
				m := make(map[string]string, len(oldTaints))
				for _, t := range oldTaints {
					m[t.Key] = t.Value + ":" + t.Effect
				}
				return m
			}(),
			func() map[string]string {
				m := make(map[string]string, len(newTaints))
				for _, t := range newTaints {
					m[t.Key] = t.Value + ":" + t.Effect
				}
				return m
			}(),
		)
		for _, k := range allKeys {
			oldT, hadOld := oldByKey[k]
			newT, hasNew := newByKey[k]
			oldStr := fmt.Sprintf("%s=%s:%s", oldT.Key, oldT.Value, oldT.Effect)
			newStr := fmt.Sprintf("%s=%s:%s", newT.Key, newT.Value, newT.Effect)
			switch {
			case hadOld && !hasNew:
				b.WriteString(removedStyle.Render(fmt.Sprintf("    - %s", oldStr)))
			case !hadOld && hasNew:
				b.WriteString(addedStyle.Render(fmt.Sprintf("    + %s", newStr)))
			case hadOld && hasNew && oldT != newT:
				b.WriteString(changedStyle.Render(fmt.Sprintf("    ! %s → %s", oldStr, newStr)))
			}
			b.WriteString("\n")
		}
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

// boolToStr converts a bool to a human-readable string.
// initialUpgradeSettings returns the upgrade-settings baseline used by both
// computeEdit and renderDiff. When the pool has no UpgradeSettings (or an
// unknown Strategy that the curated dropdown can't represent), the baseline
// is the dropdown default (SURGE/1/0). This keeps "user didn't touch the
// dropdown" === "no diff", even if the underlying pool reports an exotic
// strategy that we can't preserve.
func (v *GKENodePoolEditView) initialUpgradeSettings() (strategy string, maxSurge, maxUnavailable int64) {
	strategy = "SURGE"
	maxSurge = 1
	if v.pool.UpgradeSettings != nil {
		switch v.pool.UpgradeSettings.Strategy {
		case "SURGE", "BLUE_GREEN":
			strategy = v.pool.UpgradeSettings.Strategy
		}
		maxSurge = v.pool.UpgradeSettings.MaxSurge
		maxUnavailable = v.pool.UpgradeSettings.MaxUnavailable
	}
	return strategy, maxSurge, maxUnavailable
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
