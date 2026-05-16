package views

import (
	gocontext "context"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
)

// gkeNodes is the Nodes sub-view: a flat table over every node across
// every pool in the cluster, derived by fanning out ListManagedInstances
// over each (pool × pool.Locations × pool.InstanceGroupUrls) tuple.
type gkeNodes struct {
	projectID     string
	details       *gcp.ClusterDetails
	computeClient *gcp.ComputeClient

	table   table.Model
	spinner spinner.Model

	nodes         []gcp.GKENode // accumulated across pool fetches
	pendingMIGs   int           // counts outstanding ListManagedInstances calls
	warnings      map[string]error
	loading       bool
	err           error
	tabActive     bool
	width, height int
}

func newGKENodes(projectID string, details *gcp.ClusterDetails, computeClient *gcp.ComputeClient) *gkeNodes {
	columns := []table.Column{
		{Title: "Name", Width: 40},
		{Title: "Pool", Width: 16},
		{Title: "Zone", Width: 18},
		{Title: "Status", Width: 14},
		{Title: "Internal IP", Width: 16},
		{Title: "Age", Width: 8},
	}
	t := table.NewWithColumns(columns, "")
	return &gkeNodes{
		projectID:     projectID,
		details:       details,
		computeClient: computeClient,
		table:         t,
		spinner:       components.NewGCPSpinner(),
		warnings:      map[string]error{},
	}
}

func (s *gkeNodes) SetTabActive(active bool) { s.tabActive = active }

func (s *gkeNodes) Init() tea.Cmd {
	s.loading = true
	s.err = nil
	s.nodes = nil
	s.pendingMIGs = 0
	s.warnings = map[string]error{}
	return tea.Batch(s.spinner.Tick, s.fanOut())
}

// fanOut emits one tea.Cmd per (pool, zone, migURL) tuple. Each cmd
// returns gkeNodesPoolLoadedMsg with that MIG's nodes (pool name stamped
// from loop context) or gkeNodesPoolErrorMsg on failure.
func (s *gkeNodes) fanOut() tea.Cmd {
	if s.details == nil || s.computeClient == nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, pool := range s.details.NodePools {
		for _, migURL := range pool.InstanceGroupUrls {
			zone, migName := parseMIGURL(migURL)
			if zone == "" || migName == "" {
				continue
			}
			s.pendingMIGs++
			poolName := pool.Name
			cmds = append(cmds, s.fetchMIG(poolName, zone, migName))
		}
	}
	if len(cmds) == 0 {
		s.loading = false
		return nil
	}
	return tea.Batch(cmds...)
}

func (s *gkeNodes) fetchMIG(poolName, zone, migName string) tea.Cmd {
	return func() tea.Msg {
		ctx := gocontext.Background()
		mis, err := s.computeClient.ListManagedInstances(ctx, s.projectID, zone, migName)
		if err != nil {
			return gkeNodesPoolErrorMsg{poolName: poolName, err: err}
		}
		nodes := make([]gcp.GKENode, 0, len(mis))
		for _, mi := range mis {
			nodes = append(nodes, gcp.GKENode{MIGInstance: mi, Pool: poolName})
		}
		return gkeNodesPoolLoadedMsg{poolName: poolName, nodes: nodes}
	}
}

// parseMIGURL extracts (zone, name) from a MIG URL of the form
// .../zones/{zone}/instanceGroupManagers/{name}.
func parseMIGURL(url string) (zone, name string) {
	parts := strings.Split(url, "/")
	for i, p := range parts {
		switch p {
		case "zones":
			if i+1 < len(parts) {
				zone = parts[i+1]
			}
		case "instanceGroupManagers":
			if i+1 < len(parts) {
				name = parts[i+1]
			}
		}
	}
	return zone, name
}

// Stub Update/View/Refresh/SetSize/HasTextInputFocused — replaced in Task 7.
func (s *gkeNodes) Update(msg tea.Msg) tea.Cmd { _ = msg; return nil } // TODO: Task 7
func (s *gkeNodes) View() string               { return "" }           // TODO: Task 7
func (s *gkeNodes) Refresh() tea.Cmd           { return s.Init() }
func (s *gkeNodes) SetSize(w, h int) {
	s.width = w
	s.height = h
	s.table.SetSize(w, h-4)
}
func (s *gkeNodes) HasTextInputFocused() bool { return s.table.HasTextInputFocused() }

// dedupeAndSort and sort import are pre-staged for Task 7. Suppress
// unused-import errors by referencing them in a no-op helper if needed.
var _ = sort.Slice
