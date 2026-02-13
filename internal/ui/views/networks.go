package views

import (
	gocontext "context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
)

// NetworksView displays VPC networks in a table format
type NetworksView struct {
	TableClickDelegate
	computeClient *gcp.ComputeClient
	projectID     string
	ctx           *context.ProgramContext
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	networks      []gcp.Network
	keys          networkKeyMap

	width  int
	height int
}

// NetworkSelectedMsg is sent when a network is selected from the list
type NetworkSelectedMsg struct {
	Network gcp.Network
}

// networkKeyMap defines network-specific key bindings
type networkKeyMap struct {
	Select  key.Binding
	Refresh key.Binding
}

func defaultNetworkKeyMap() networkKeyMap {
	return networkKeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "details"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// Table column definitions
func networkColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 35, Grow: true, Sortable: true},
		{Title: "Subnet Mode", Width: 12, Sortable: true},
		{Title: "Routing", Width: 10},
		{Title: "Subnets", Width: 8, Sortable: true},
		{Title: "Created", Width: 20},
	}
}

// NewNetworksView creates a new networks view with table display
func NewNetworksView(projectID string) *NetworksView {
	title := fmt.Sprintf("VPC Networks - %s", projectID)
	t := table.NewWithColumns(networkColumns(), title)

	s := components.NewGCPSpinner()

	v := &NetworksView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultNetworkKeyMap(),
	}
	v.Table = &v.table
	return v
}

// Init initializes the view and starts loading networks
func (v *NetworksView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Tick,
		v.initComputeClient(),
	)
}

// initComputeClient creates the compute client then loads networks
func (v *NetworksView) initComputeClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewComputeClient(gocontext.Background())
		if err != nil {
			return networksErrorMsg{err: err}
		}
		return networksClientReadyMsg{client: client}
	}
}

// loadNetworks fetches VPC networks from GCP
func (v *NetworksView) loadNetworks() tea.Cmd {
	return func() tea.Msg {
		networks, err := v.computeClient.ListNetworks(gocontext.Background(), v.projectID)
		if err != nil {
			return networksErrorMsg{err: err}
		}
		return networksLoadedMsg{networks: networks}
	}
}

// Message types
type networksClientReadyMsg struct {
	client *gcp.ComputeClient
}

type networksLoadedMsg struct {
	networks []gcp.Network
}

type networksErrorMsg struct {
	err error
}

// networkToRow converts a GCP network to a table row
func networkToRow(network gcp.Network) table.Row { //nolint:gocritic // Copying network is acceptable
	subnetMode := "Auto"
	if !network.AutoCreate {
		subnetMode = "Custom"
	}

	return table.Row{
		Data: []string{
			network.Name,
			subnetMode,
			network.RoutingMode,
			fmt.Sprintf("%d", network.SubnetsCount),
			network.CreatedAt,
		},
		FilterValue: network.Name + " " + subnetMode + " " + network.RoutingMode,
		ID:          network.Name,
	}
}

// Update handles messages for the networks view
func (v *NetworksView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case networksClientReadyMsg:
		v.computeClient = msg.client
		return v.loadNetworks()

	case networksLoadedMsg:
		v.loading = false
		v.networks = msg.networks

		rows := make([]table.Row, len(msg.networks))
		for i, network := range msg.networks {
			rows[i] = networkToRow(network)
		}
		v.table.SetRows(rows)
		return nil

	case networksErrorMsg:
		v.loading = false
		v.err = msg.err
		return nil

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		if v.loading {
			return nil
		}

		// Delegate to table when sort menu is open
		if v.table.IsSortMenuOpen() {
			var cmd tea.Cmd
			v.table, cmd = v.table.Update(msg)
			return cmd
		}

		// Let table handle filtering mode
		if v.table.IsFiltering() {
			var cmd tea.Cmd
			v.table, cmd = v.table.Update(msg)
			return cmd
		}

		switch {
		case key.Matches(msg, v.keys.Select):
			if network, ok := v.findNetworkByID(v.table.SelectedRow().ID); ok {
				return func() tea.Msg { return NetworkSelectedMsg{Network: network} }
			}
			return nil

		case key.Matches(msg, v.keys.Refresh):
			v.loading = true
			v.err = nil
			// Re-initialize client if previous attempt failed
			if v.computeClient == nil {
				return tea.Batch(v.spinner.Tick, v.initComputeClient())
			}
			return tea.Batch(v.spinner.Tick, v.loadNetworks())
		}
	}

	// Update table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// HasTextInputFocused returns true if the table filter is active.
func (v *NetworksView) HasTextInputFocused() bool {
	return v.table.HasTextInputFocused()
}

// View renders the networks view
func (v *NetworksView) View() string {
	if v.loading && v.computeClient == nil {
		return renderLoading(v.spinner, "Initializing Compute Engine client...")
	}

	if v.loading {
		return renderLoading(v.spinner, "Loading VPC networks...")
	}

	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}

	if len(v.networks) == 0 {
		return "\n  No VPC networks found in this project.\n  Press 'esc' to go back."
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • S: sort • /: filter • r: refresh • esc: back")

	return v.table.View() + help
}

// findNetworkByID finds a network in the loaded list by name (used as ID)
func (v *NetworksView) findNetworkByID(id string) (gcp.Network, bool) {
	for _, n := range v.networks {
		if n.Name == id {
			return n, true
		}
	}
	return gcp.Network{}, false
}

// GetComputeClient returns the compute client for reuse in detail views
func (v *NetworksView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}

// SetContext updates the view with shared program context.
func (v *NetworksView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

