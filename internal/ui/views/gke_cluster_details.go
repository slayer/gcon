// internal/ui/views/gke_cluster_details.go
package views

import (
	gocontext "context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	// "github.com/slayer/gcon/internal/ui/components/links"           // add when Task 12 needs it
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
	"github.com/slayer/gcon/internal/ui/overlay"
)

// GKEClusterDetailsView shows a single GKE cluster's overview and node pools.
// Phase 1 surface: read-only Overview + Node Pools tabs and a Delete action
// (wired in Task 14).
type GKEClusterDetailsView struct {
	projectID     string
	location      string
	name          string
	client        *gcp.ContainerClient
	computeClient *gcp.ComputeClient // for cross-view nav (Network / Subnet links)
	details       *gcp.ClusterDetails
	tabs          *tabs.Tabs
	viewport      viewport.Model
	viewportSize  bool
	spinner       spinner.Model
	width, height int

	// Node Pools tab
	poolsTable table.Model

	// Delete dialog
	confirmDialog *confirm.TypeConfirmDialog
	showConfirm   bool
	deleting      bool

	err     error
	loading bool
}

// NewGKEClusterDetailsView constructs the details view. The container client
// may be nil; it will be lazily created on Init.
func NewGKEClusterDetailsView(projectID, location, name string, container *gcp.ContainerClient, compute *gcp.ComputeClient) *GKEClusterDetailsView {
	v := &GKEClusterDetailsView{
		projectID:     projectID,
		location:      location,
		name:          name,
		client:        container,
		computeClient: compute,
		spinner:       components.NewGCPSpinner(),
		tabs: tabs.New([]tabs.Tab{
			{ID: "overview", Label: "Overview"},
			{ID: "nodepools", Label: "Node Pools"},
		}),
	}
	v.poolsTable = table.NewWithColumns([]table.Column{
		{Title: "Name", Width: 40},
		{Title: "Machine type", Width: 18},
		{Title: "Nodes", Width: 8},
		{Title: "Autoscale", Width: 14},
		{Title: "Version", Width: 22},
		{Title: "Status", Width: 14},
		{Title: "Auto-upgrade", Width: 12},
		{Title: "Auto-repair", Width: 12},
	}, "")
	return v
}

// Init kicks off the lazy client create (if needed) and the cluster fetch.
func (v *GKEClusterDetailsView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
	if v.client == nil {
		return tea.Batch(v.spinner.Tick, v.initClient())
	}
	return tea.Batch(v.spinner.Tick, v.load())
}

func (v *GKEClusterDetailsView) initClient() tea.Cmd {
	return func() tea.Msg {
		c, err := gcp.NewContainerClient(gocontext.Background())
		if err != nil {
			return gkeClusterErrorMsg{err: err}
		}
		return gkeClusterClientReadyMsg{client: c}
	}
}

func (v *GKEClusterDetailsView) load() tea.Cmd {
	return func() tea.Msg {
		if v.client == nil {
			return gkeClusterErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		d, err := v.client.GetCluster(gocontext.Background(), v.projectID, v.location, v.name)
		if err != nil {
			return gkeClusterErrorMsg{err: err}
		}
		return gkeClusterLoadedMsg{details: d}
	}
}

// Name returns the cluster name (used for breadcrumbs / page title).
func (v *GKEClusterDetailsView) Name() string { return v.name }

// GetComputeClient exposes the compute client for cross-view nav handlers.
func (v *GKEClusterDetailsView) GetComputeClient() *gcp.ComputeClient { return v.computeClient }

// SetSize updates the inner viewport and node pools table to match the
// available content area. Leaves 4 rows for the tab bar + status line.
func (v *GKEClusterDetailsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.poolsTable.SetSize(width-4, height-8)
	if !v.viewportSize {
		v.viewport = viewport.New(width-4, height-4)
		v.viewportSize = true
	} else {
		v.viewport.Width = width - 4
		v.viewport.Height = height - 4
	}
}

// SetContext is a no-op for now; details views don't need ProgramContext.
func (v *GKEClusterDetailsView) SetContext(_ *context.ProgramContext) {}

// HasTextInputFocused reports whether a text input owns the keyboard.
// Returns true when the delete confirmation dialog is open so the global
// 'q'-to-quit binding can't fire while the user is typing the cluster name.
func (v *GKEClusterDetailsView) HasTextInputFocused() bool {
	if v.showConfirm && v.confirmDialog != nil {
		return v.confirmDialog.HasTextInputFocused()
	}
	return false
}

// IsMenuOpen reports whether a dialog is open so the app's Esc handler
// routes Esc back to the view (which then closes the dialog) instead of
// navigating back.
func (v *GKEClusterDetailsView) IsMenuOpen() bool {
	return v.showConfirm
}

// SetError lets the app propagate async errors (e.g. delete failures) back
// into the view so they render inline.
func (v *GKEClusterDetailsView) SetError(err error) {
	v.deleting = false
	v.err = err
}

func (v *GKEClusterDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case gkeClusterClientReadyMsg:
		v.client = m.client
		return v.load()
	case gkeClusterLoadedMsg:
		v.loading = false
		v.details = m.details
		// Refresh the node pools table now that data is loaded.
		v.refreshNodePoolsTable()
		return nil
	case gkeClusterErrorMsg:
		v.loading = false
		v.err = m.err
		return nil
	case GKEClusterActionResultMsg:
		v.deleting = false
		if m.Error != nil {
			v.err = m.Error
		}
		return nil
	case tabs.TabChangedMsg:
		// Reset scroll so each tab opens at the top.
		if v.viewportSize {
			v.viewport.GotoTop()
		}
		return nil
	case confirm.TypeConfirmMsg:
		// User typed the cluster name and pressed Enter. Close the dialog,
		// flip into deleting state, and emit the request for the app handler.
		v.showConfirm = false
		if v.details == nil {
			return nil
		}
		v.deleting = true
		name := v.details.Name
		location := v.details.Location
		projectID := v.projectID
		return func() tea.Msg {
			return GKEClusterDeleteRequestMsg{
				ProjectID: projectID,
				Location:  location,
				Name:      name,
			}
		}
	case confirm.TypeCancelMsg:
		v.showConfirm = false
		v.confirmDialog = nil
		return nil
	case spinner.TickMsg:
		// Suppress ticks when no async work is in flight — otherwise the
		// spinner keeps re-emitting ticks forever and drives continuous
		// redraws even when nothing is loading or deleting.
		if !v.loading && !v.deleting {
			return nil
		}
		var cmd tea.Cmd
		v.spinner, cmd = v.spinner.Update(m)
		return cmd
	case tea.KeyMsg:
		return v.handleKey(m)
	}
	return nil
}

func (v *GKEClusterDetailsView) handleKey(m tea.KeyMsg) tea.Cmd {
	// Route keys to the delete dialog while it's open.
	if v.showConfirm && v.confirmDialog != nil {
		return v.confirmDialog.Update(m)
	}
	switch m.String() {
	case "D":
		v.openDeleteDialog()
		if v.confirmDialog != nil {
			return v.confirmDialog.Init()
		}
		return nil
	case "r":
		v.loading = true
		v.err = nil
		if v.client == nil {
			return tea.Batch(v.spinner.Tick, v.initClient())
		}
		return tea.Batch(v.spinner.Tick, v.load())
	case "tab", "shift+tab", "h", "l", "left", "right", "1", "2":
		// Delegate tab navigation to the embedded tabs widget; it emits
		// TabChangedMsg which the view's Update handles to reset scroll.
		return v.tabs.Update(m)
	}
	// Node Pools tab consumes j/k/up/down/enter for its own cursor; the
	// viewport only scrolls on keys the table doesn't handle.
	if v.tabs.ActiveTab().ID == "nodepools" && isPoolsTableKey(m) {
		var cmd tea.Cmd
		v.poolsTable, cmd = v.poolsTable.Update(m)
		return cmd
	}
	if isViewportScrollKey(m) {
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(m)
		return cmd
	}
	return nil
}

// isPoolsTableKey reports whether a key should be routed to the Node Pools
// table for cursor movement instead of scrolling the surrounding viewport.
// PgUp/PgDn/Home/End keep going to the viewport (the table is short).
func isPoolsTableKey(m tea.KeyMsg) bool {
	switch m.String() {
	case "j", "k", "up", "down", "enter":
		return true
	}
	return false
}

// openDeleteDialog constructs and shows the type-to-confirm delete dialog.
// Body text spells out the side-effects that GKE does NOT auto-clean (PVs
// with reclaimPolicy=Retain, external load balancers, Cloud DNS entries).
func (v *GKEClusterDetailsView) openDeleteDialog() {
	if v.details == nil {
		return
	}
	d := v.details
	detailLines := []string{
		fmt.Sprintf("Location: %s (%s)", d.Location, humanLocationType(d.LocationType)),
		fmt.Sprintf("Mode: %s", humanMode(d.Mode)),
		fmt.Sprintf("Node pools: %d", len(d.NodePools)),
		"",
		"This will permanently delete the cluster, its node pools,",
		"and all workloads running in it. The operation takes 2–5",
		"minutes and runs server-side after this call returns.",
		"",
		"NOT auto-deleted by GKE:",
		"  • Persistent volumes from dynamic provisioning unless",
		"    the StorageClass uses reclaimPolicy=Delete",
		"  • External load balancers created by Service of type",
		"    LoadBalancer (deleted only if cluster removal succeeds)",
		"  • Cloud DNS entries managed by the cluster",
	}
	v.confirmDialog = confirm.NewTypeConfirmDialog(
		"Delete GKE Cluster",
		d.Name,
		detailLines,
	)
	v.showConfirm = true
}

func (v *GKEClusterDetailsView) View() string {
	if v.loading && v.details == nil {
		return renderLoading(v.spinner, "Loading cluster...")
	}
	if v.err != nil && v.details == nil {
		return components.RenderError(v.err)
	}
	var b strings.Builder
	b.WriteString(v.tabs.View())
	b.WriteString("\n\n")
	var body string
	switch v.tabs.ActiveTab().ID {
	case "overview":
		body = v.renderOverview()
	case "nodepools":
		body = v.renderNodePools()
	}
	if v.viewportSize {
		v.viewport.SetContent(body)
		b.WriteString(v.viewport.View())
	} else {
		b.WriteString(body)
	}
	if v.deleting {
		b.WriteString("\n\nDeleting cluster...\n")
	}
	// Inline action error (e.g. delete failure) — renders below the body.
	if v.err != nil && v.details != nil {
		b.WriteString("\n")
		b.WriteString(components.RenderInlineError(v.err))
	}
	mainContent := b.String()

	// Overlay the delete dialog on top of the main content. Per the
	// bubble-tea-rendering rule on dialog z-order, dialogs must render
	// ABOVE the parent body.
	if v.showConfirm && v.confirmDialog != nil {
		contentHeight := lipgloss.Height(mainContent)
		return overlay.Center(mainContent, v.confirmDialog.View(), v.width, contentHeight)
	}
	return mainContent
}

func (v *GKEClusterDetailsView) renderOverview() string {
	d := v.details
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(24)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))
	var b strings.Builder
	fmt.Fprintf(&b, "Cluster: %s\n", d.Name)
	b.WriteString(strings.Repeat("─", 60) + "\n")
	row := func(k, val string) {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, k, val))
	}
	row("Mode", humanMode(d.Mode))
	row("Status", statusBadge(d.Status))
	row("Location", fmt.Sprintf("%s (%s)", d.Location, humanLocationType(d.LocationType)))
	row("Master version", d.MasterVersion)
	nv := d.NodeVersion
	if !d.NodeVersionsUniform {
		nv = "(varies)"
	}
	row("Node version", nv)
	row("Release channel", defaultIfEmpty(d.ReleaseChannel, "(unspecified)"))
	row("Created", d.CreatedAt)

	b.WriteString("\nNetworking\n")
	b.WriteString(strings.Repeat("─", 60) + "\n")
	row("Network", d.Network)
	row("Subnetwork", d.Subnetwork)
	row("Cluster IPv4 CIDR", d.ClusterIPv4CIDR)
	row("Services IPv4 CIDR", d.ServicesIPv4CIDR)
	row("Endpoint", d.Endpoint)
	row("Private cluster", yesNo(d.PrivateCluster))

	b.WriteString("\nSecurity\n")
	b.WriteString(strings.Repeat("─", 60) + "\n")
	row("Workload Identity", defaultIfEmpty(d.WorkloadIdentityPool, "(off)"))
	row("Database encryption", formatDatabaseEncryption(d.DatabaseEncrypted, d.DatabaseKMSKey))
	authNets := "(none)"
	if len(d.MasterAuthorizedNetworks) > 0 {
		authNets = strings.Join(d.MasterAuthorizedNetworks, ", ")
	}
	row("Authorized networks", authNets)

	b.WriteString("\nAdd-ons\n")
	b.WriteString(strings.Repeat("─", 60) + "\n")
	addon := func(k string, on bool) {
		state := "Disabled"
		if on {
			state = "Enabled"
		}
		fmt.Fprintf(&b, "  %s\n", mutedStyle.Render(fmt.Sprintf("%s: %s", k, state)))
	}
	addon("HTTP load balancing", d.Addons.HTTPLoadBalancing)
	addon("Network policy", d.Addons.NetworkPolicy)
	addon("Persistent disk CSI", d.Addons.PersistentDiskCSI)
	addon("DNS cache", d.Addons.DNSCache)
	return b.String()
}

func (v *GKEClusterDetailsView) renderNodePools() string {
	if v.details == nil {
		return ""
	}
	if len(v.details.NodePools) == 0 {
		return "(no node pools)"
	}
	return v.poolsTable.View()
}

func (v *GKEClusterDetailsView) refreshNodePoolsTable() {
	if v.details == nil {
		return
	}
	autopilot := v.details.Mode == "AUTOPILOT"
	rows := make([]table.Row, 0, len(v.details.NodePools))
	for i := range v.details.NodePools {
		p := &v.details.NodePools[i]
		nameCell := p.Name
		autoscale := "off"
		version := p.NodeVersion
		if p.AutoscalingOn {
			autoscale = fmt.Sprintf("on (%d–%d)", p.AutoscalingMin, p.AutoscalingMax)
		}
		if autopilot {
			// Plain (unstyled) suffix: ANSI bytes inflate the cell's byte
			// length and trip bubbles/table's column-width truncation, which
			// would clip the "[managed by Autopilot]" marker mid-word.
			nameCell = p.Name + " [managed by Autopilot]"
			autoscale = "—"
			version = "—"
		}
		rows = append(rows, table.Row{
			ID: p.Name,
			Data: []string{
				nameCell,
				p.MachineType,
				fmt.Sprintf("%d", p.NodeCount),
				autoscale,
				version,
				statusBadge(p.Status),
				checkmark(p.AutoUpgrade),
				checkmark(p.AutoRepair),
			},
			FilterValue: strings.Join([]string{
				p.Name, p.MachineType, autoscale, version, p.Status,
			}, " "),
		})
	}
	v.poolsTable.SetRows(rows)
}

func checkmark(b bool) string {
	if b {
		return "✓"
	}
	return ""
}

func humanMode(m string) string {
	switch m {
	case "AUTOPILOT":
		return "Autopilot"
	case "STANDARD":
		return "Standard"
	}
	return m
}

// humanLocationType converts the raw LocationType ("zone" / "region")
// into its adjectival form ("zonal" / "regional") for display.
func humanLocationType(t string) string {
	switch t {
	case "zone":
		return "zonal"
	case "region":
		return "regional"
	}
	return t
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// formatDatabaseEncryption composes the human-readable encryption label.
// keyURI is the full KMS resource URI; only the last segment is shown.
func formatDatabaseEncryption(encrypted bool, keyURI string) string {
	if !encrypted {
		return "DECRYPTED"
	}
	if keyURI == "" {
		return "ENCRYPTED"
	}
	// Extract the last "/"-segment as the short key name.
	idx := strings.LastIndex(keyURI, "/")
	if idx == -1 {
		return fmt.Sprintf("ENCRYPTED (key: %s)", keyURI)
	}
	return fmt.Sprintf("ENCRYPTED (key: %s)", keyURI[idx+1:])
}
