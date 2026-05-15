// internal/ui/views/gke_clusters.go
package views

import (
	gocontext "context"

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
		{Title: "Nodes", Width: 8},
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

// TODO: Task 10 — replace these stubs with real Update / View / refreshTable.
func (v *GKEClustersView) Update(msg tea.Msg) tea.Cmd { _ = msg; return nil }

// TODO: Task 10 — replace with real rendering.
func (v *GKEClustersView) View() string { return "" }
