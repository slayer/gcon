package views

import (
	gocontext "context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/symbols"
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
	for i := range s.details.NodePools {
		pool := &s.details.NodePools[i]
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

func (s *gkeNodes) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case gkeNodesPoolLoadedMsg:
		s.pendingMIGs--
		s.nodes = append(s.nodes, m.nodes...)
		s.dedupeAndSort()
		s.refreshTable()
		if s.pendingMIGs <= 0 {
			s.loading = false
		}
		return nil
	case gkeNodesPoolErrorMsg:
		s.pendingMIGs--
		s.warnings[m.poolName] = m.err
		if s.pendingMIGs <= 0 {
			s.loading = false
		}
		return nil
	case spinner.TickMsg:
		if !s.loading {
			return nil
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(m)
		return cmd
	case tea.KeyMsg:
		return s.handleKey(m)
	}
	return nil
}

func (s *gkeNodes) handleKey(m tea.KeyMsg) tea.Cmd {
	if m.String() == "enter" {
		n := s.cursorNode()
		if n == nil {
			return nil
		}
		// Emit InstanceSelectedMsg with the node's Name and Zone. The
		// app-level handler (handleInstanceSelected) only reads Name and
		// Zone, so a minimal gcp.Instance is sufficient.
		return func() tea.Msg {
			return InstanceSelectedMsg{Instance: gcp.Instance{
				Name: n.Name,
				Zone: n.Zone,
			}}
		}
	}
	_, cmd := s.table.Update(m)
	return cmd
}

func (s *gkeNodes) View() string {
	if s.loading && len(s.nodes) == 0 {
		return renderLoading(s.spinner, "Loading nodes...")
	}
	if s.err != nil && len(s.nodes) == 0 {
		return components.RenderError(s.err)
	}
	out := s.table.View()
	if len(s.warnings) > 0 {
		muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
		// Sort pool names for deterministic warning ordering.
		poolNames := make([]string, 0, len(s.warnings))
		for pool := range s.warnings {
			poolNames = append(poolNames, pool)
		}
		sort.Strings(poolNames)
		var b strings.Builder
		b.WriteString("\n")
		for _, pool := range poolNames {
			b.WriteString(muted.Render(fmt.Sprintf("  ⚠ %s: %v", pool, s.warnings[pool])))
			b.WriteString("\n")
		}
		out += b.String()
	}
	return out
}

func (s *gkeNodes) refreshTable() {
	rows := make([]table.Row, 0, len(s.nodes))
	for i := range s.nodes {
		n := &s.nodes[i]
		rows = append(rows, table.Row{
			ID: n.Zone + "/" + n.Name,
			Data: []string{
				n.Name,
				n.Pool,
				n.Zone,
				gkeNodeStatusBadge(n.Status),
				defaultIfEmpty(n.InternalIP, "-"),
				formatAge(n.CreatedAt),
			},
			FilterValue: strings.Join([]string{n.Name, n.Pool, n.Zone, n.Status, n.InternalIP}, " "),
		})
	}
	s.table.SetRows(rows)
}

func (s *gkeNodes) dedupeAndSort() {
	seen := make(map[string]struct{}, len(s.nodes))
	out := make([]gcp.GKENode, 0, len(s.nodes))
	for _, n := range s.nodes {
		key := n.Zone + "/" + n.Name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	s.nodes = out
}

func (s *gkeNodes) cursorNode() *gcp.GKENode {
	row := s.table.SelectedRow()
	if row == nil {
		return nil
	}
	for i := range s.nodes {
		n := &s.nodes[i]
		if n.Zone+"/"+n.Name == row.ID {
			return n
		}
	}
	return nil
}

// gkeNodeStatusBadge uses the symbols package for the status indicator
// (emoji glyph in default mode), avoiding the bubbles/table ANSI byte-
// truncation issue caught in Phase 1.
func gkeNodeStatusBadge(status string) string {
	if status == "" {
		return "—"
	}
	return symbols.GetStatusSymbol(status) + " " + status
}

// formatAge renders an RFC3339 CreationTimestamp as a short age string.
func formatAge(rfc3339 string) string {
	if rfc3339 == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	default:
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
}

func (s *gkeNodes) Refresh() tea.Cmd { return s.Init() }
func (s *gkeNodes) SetSize(w, h int) {
	s.width = w
	s.height = h
	s.table.SetSize(w, h-4)
}
func (s *gkeNodes) HasTextInputFocused() bool { return s.table.HasTextInputFocused() }
