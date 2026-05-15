// internal/ui/views/gke_cluster_details.go
package views

import (
	gocontext "context"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	// "github.com/slayer/gcon/internal/ui/components/confirmdialog"  // add when Task 14 needs it
	// "github.com/slayer/gcon/internal/ui/components/links"           // add when Task 12 needs it
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
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

	// Delete dialog (wired in Task 14)
	deleting bool

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
		{Title: "Name", Width: 18},
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
// Task 14 will return v.confirmDialog.HasTextInputFocused() when the dialog
// is open.
func (v *GKEClusterDetailsView) HasTextInputFocused() bool {
	return false
}

// SetError lets the app propagate async errors (e.g. delete failures) back
// into the view so they render inline.
func (v *GKEClusterDetailsView) SetError(err error) {
	v.deleting = false
	v.err = err
}

func (v *GKEClusterDetailsView) Update(msg tea.Msg) tea.Cmd { _ = msg; return nil } // TODO: Task 12
func (v *GKEClusterDetailsView) View() string               { return "" }           // TODO: Task 12
