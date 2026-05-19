package views

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/forms"
	"github.com/slayer/gcon/internal/ui/context"
)

// nodePoolEditState represents the view lifecycle.
type nodePoolEditState int

const (
	nodePoolEditStateForm    nodePoolEditState = iota
	nodePoolEditStateDiff                      // preview old vs new
	nodePoolEditStateSaving                    // calling API (spinner)
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

// GKENodePoolEditView is a 3-state form view for editing a GKE node pool.
// States: form → diff (preview) → saving.
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
	// k8s labels on NodeConfig — read-only in MVP.
	labelsDisplay := nodePoolLabelsToDisplay(v.pool.Labels)
	labelsSection := forms.NewSection("labels", "Node Labels").
		AddField(forms.NewReadOnlyField("node_labels", "Node Labels", labelsDisplay).
			SetHelpText("Label editing planned for a future PR"))

	v.Form.AddSection(labelsSection)

	// ── Taints section ────────────────────────────────────────────────────────
	// Node taints — read-only in MVP.
	taintsDisplay := taintsToDisplay(v.pool.Taints)
	taintsSection := forms.NewSection("taints", "Node Taints").
		AddField(forms.NewReadOnlyField("node_taints", "Node Taints", taintsDisplay).
			SetHelpText("Taint editing planned for a future PR"))

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

	if v.pool.UpgradeSettings == nil {
		upgradeHelpText = "Pool currently has no explicit upgrade settings; submitting will set defaults."
	} else {
		if v.pool.UpgradeSettings.Strategy != "" {
			initialStrategy = v.pool.UpgradeSettings.Strategy
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

// nodePoolLabelsToDisplay formats a node pool label map as "key=value, ..." sorted alphabetically.
// Returns "(none)" when empty.
func nodePoolLabelsToDisplay(labels map[string]string) string {
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
}

// SetContext implements the View interface.
func (v *GKENodePoolEditView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.SetSize(ctx.ContentWidth, ctx.ContentHeight)
}

// HasTextInputFocused returns true when a text input is active (form state only).
func (v *GKENodePoolEditView) HasTextInputFocused() bool {
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
	// Determine initial values (handle nil UpgradeSettings).
	initialStrategy := "SURGE"
	var initialMaxSurge int64 = 1
	var initialMaxUnavailable int64

	if v.pool.UpgradeSettings != nil {
		if v.pool.UpgradeSettings.Strategy != "" {
			initialStrategy = v.pool.UpgradeSettings.Strategy
		}
		initialMaxSurge = v.pool.UpgradeSettings.MaxSurge
		initialMaxUnavailable = v.pool.UpgradeSettings.MaxUnavailable
	}

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

	return fields, management, nil
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

		initialStrategy := "SURGE"
		var initialMaxSurge int64 = 1
		var initialMaxUnavailable int64
		if v.pool.UpgradeSettings != nil {
			if v.pool.UpgradeSettings.Strategy != "" {
				initialStrategy = v.pool.UpgradeSettings.Strategy
			}
			initialMaxSurge = v.pool.UpgradeSettings.MaxSurge
			initialMaxUnavailable = v.pool.UpgradeSettings.MaxUnavailable
		}

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

	if !anyChange {
		b.WriteString(mutedStyle.Render("  (no changes)"))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("  Enter: apply   Esc: back to form"))
	b.WriteString("\n")

	return b.String()
}

// boolToStr converts a bool to a human-readable string.
func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
