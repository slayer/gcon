// internal/ui/views/gke_clusters.go
package views

import (
	gocontext "context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
)

// GKEClustersView is the project-wide list of GKE clusters. It mirrors the
// shape of LoadBalancersView: lazy ContainerClient, async load, table render.
type GKEClustersView struct {
	TableClickDelegate

	projectID string
	client    *gcp.ContainerClient
	clusters  []gcp.Cluster
	table     table.Model
	spinner   spinner.Model
	loading   bool
	err       error
	width     int
	height    int
}

// NewGKEClustersView constructs the list view. If client is nil the view
// will initialize one on Init().
func NewGKEClustersView(projectID string, client *gcp.ContainerClient) *GKEClustersView {
	columns := []table.Column{
		{Title: "Name", Width: 28, Sortable: true},
		{Title: "Location", Width: 18, Sortable: true},
		{Title: "Type", Width: 8, Sortable: true},
		{Title: "Mode", Width: 10, Sortable: true},
		{Title: "Master version", Width: 22},
		{Title: "Nodes", Width: 10},
		{Title: "Status", Width: 14, Sortable: true},
	}
	t := table.NewWithColumns(columns, "Kubernetes Engine - Clusters")
	v := &GKEClustersView{
		projectID: projectID,
		client:    client,
		table:     t,
		spinner:   components.NewGCPSpinner(),
		loading:   true,
	}
	v.Table = &v.table
	return v
}

// Init kicks off the initial load.
func (v *GKEClustersView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
	if v.client == nil {
		return tea.Batch(v.spinner.Tick, v.initClient())
	}
	return tea.Batch(v.spinner.Tick, v.load())
}

func (v *GKEClustersView) initClient() tea.Cmd {
	return func() tea.Msg {
		c, err := gcp.NewContainerClient(gocontext.Background())
		if err != nil {
			return gkeClustersErrorMsg{err: err}
		}
		return gkeClustersClientReadyMsg{client: c}
	}
}

func (v *GKEClustersView) load() tea.Cmd {
	return func() tea.Msg {
		if v.client == nil {
			return gkeClustersErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		out, err := v.client.ListClusters(gocontext.Background(), v.projectID)
		if err != nil {
			return gkeClustersErrorMsg{err: err}
		}
		return gkeClustersLoadedMsg{clusters: out}
	}
}

// GetContainerClient exposes the client for cross-view reuse in detail views.
func (v *GKEClustersView) GetContainerClient() *gcp.ContainerClient { return v.client }

// SetSize records dimensions and resizes the table.
func (v *GKEClustersView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.table.SetSize(width, height-4)
}

// SetContext updates the view with shared program context. Currently no-op;
// the view sizes itself via SetSize from the app layout.
func (v *GKEClustersView) SetContext(_ *context.ProgramContext) {}

// HasTextInputFocused delegates to the table's filter input.
func (v *GKEClustersView) HasTextInputFocused() bool { return v.table.HasTextInputFocused() }

// Update routes messages for the list view.
func (v *GKEClustersView) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case gkeClustersClientReadyMsg:
		v.client = m.client
		return v.load()
	case gkeClustersLoadedMsg:
		v.loading = false
		v.err = nil
		v.clusters = m.clusters
		v.refreshTable()
		return nil
	case gkeClustersErrorMsg:
		v.loading = false
		v.err = m.err
		return nil
	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(m)
			return cmd
		}
		return nil
	case tea.KeyMsg:
		return v.handleKey(m)
	}
	return nil
}

func (v *GKEClustersView) handleKey(m tea.KeyMsg) tea.Cmd {
	if v.loading {
		return nil
	}
	// Let the table own its filter / sort menu input.
	if v.table.IsSortMenuOpen() || v.table.IsFiltering() {
		var cmd tea.Cmd
		v.table, cmd = v.table.Update(m)
		return cmd
	}
	switch m.String() {
	case "enter":
		c := v.cursorCluster()
		if c == nil {
			return nil
		}
		selected := *c
		return func() tea.Msg {
			return GKEClusterSelectedMsg{
				ProjectID: v.projectID,
				Location:  selected.Location,
				Name:      selected.Name,
			}
		}
	case "D":
		// Phase 1: list view's D navigates to details, where the delete
		// dialog lives. This keeps the dialog code in one place.
		c := v.cursorCluster()
		if c == nil {
			return nil
		}
		selected := *c
		return func() tea.Msg {
			return GKEClusterSelectedMsg{
				ProjectID: v.projectID,
				Location:  selected.Location,
				Name:      selected.Name,
			}
		}
	case "r":
		v.loading = true
		v.err = nil
		if v.client == nil {
			return tea.Batch(v.spinner.Tick, v.initClient())
		}
		return tea.Batch(v.spinner.Tick, v.load())
	}
	// Defer remaining keys (j/k, sort menu open, filter open, mouse) to the table.
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(m)
	return cmd
}

// View renders the list view.
func (v *GKEClustersView) View() string {
	if v.loading && len(v.clusters) == 0 {
		return renderLoading(v.spinner, "Loading clusters...")
	}
	if v.err != nil && len(v.clusters) == 0 {
		return "\n" + components.RenderError(v.err)
	}
	out := v.table.View()
	// If a refresh failed after a successful first load, surface the
	// error inline below the table so the user knows the visible data
	// is stale.
	if v.err != nil {
		out += components.RenderInlineError(v.err)
	}
	return out
}

// refreshTable rebuilds the table rows from the current cluster slice.
func (v *GKEClustersView) refreshTable() {
	rows := make([]table.Row, 0, len(v.clusters))
	for i := range v.clusters {
		c := &v.clusters[i]
		nodes := fmt.Sprintf("%d", c.NodeCount)
		if c.Mode == "AUTOPILOT" {
			nodes = "(managed)"
		}
		mode := modeBadge(c.Mode)
		locType := locationBadge(c.LocationType)
		status := statusBadge(c.Status)
		rows = append(rows, table.Row{
			ID: c.Location + "/" + c.Name,
			Data: []string{
				c.Name,
				c.Location,
				locType,
				mode,
				c.MasterVersion,
				nodes,
				status,
			},
			FilterValue: strings.Join([]string{
				c.Name, c.Location, locType, mode, c.MasterVersion, nodes, c.Status,
			}, " "),
		})
	}
	v.table.SetRows(rows)
}

// cursorCluster returns the cluster under the table cursor, or nil if no
// row is selected or the index is out of range.
func (v *GKEClustersView) cursorCluster() *gcp.Cluster {
	idx := v.table.SelectedIndex()
	if idx < 0 || idx >= len(v.clusters) {
		return nil
	}
	return &v.clusters[idx]
}

// locationBadge converts the raw location-type string ("zone" / "region")
// into a slightly more readable badge for the Type column.
func locationBadge(kind string) string {
	switch kind {
	case "zone":
		return "zonal"
	case "region":
		return "regional"
	}
	return ""
}

// modeBadge formats the cluster mode (STANDARD / AUTOPILOT) for display.
func modeBadge(mode string) string {
	switch mode {
	case "AUTOPILOT":
		return "Autopilot"
	case "STANDARD":
		return "Standard"
	}
	return mode
}

// statusBadge prefixes the cluster status with a status dot. The dot is
// intentionally NOT lipgloss-styled: bubbles/table truncates cells by
// byte length rather than visual width, so embedding ANSI escapes here
// causes the table to slice through the escape sequence mid-cell and
// strip the visible content. Color is sacrificed so the column renders.
// If the status is empty (some GKE responses omit it for active
// clusters), fall back to "—" so the column still has content.
func statusBadge(status string) string {
	if status == "" {
		return "—"
	}
	return "● " + status
}
