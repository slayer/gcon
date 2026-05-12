package views

import (
	gocontext "context"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
)

// LoadBalancersView is the project-wide list of forwarding rules.
type LoadBalancersView struct {
	TableClickDelegate

	projectID string
	client    *gcp.ComputeClient
	ctx       *context.ProgramContext

	spinner spinner.Model
	loading bool
	err     error
	rules   []gcp.ForwardingRule
	table   table.Model

	width, height int
	keys          loadBalancersKeyMap
}

type loadBalancersKeyMap struct {
	Select  key.Binding
	Refresh key.Binding
}

func defaultLoadBalancersKeyMap() loadBalancersKeyMap {
	return loadBalancersKeyMap{
		Select:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}

func loadBalancerColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 24, Grow: true, Sortable: true},
		{Title: "Type", Width: 24, Sortable: true},
		{Title: "Scope", Width: 16, Sortable: true},
		{Title: "IP", Width: 18},
		{Title: "Ports", Width: 14},
		{Title: "Backend", Width: 24},
	}
}

// NewLoadBalancersView constructs the list view. If client is nil the view
// will initialize one on Init().
func NewLoadBalancersView(projectID string, client *gcp.ComputeClient) *LoadBalancersView {
	t := table.NewWithColumns(loadBalancerColumns(), "Load balancers - "+projectID)

	v := &LoadBalancersView{
		projectID: projectID,
		client:    client,
		spinner:   components.NewGCPSpinner(),
		loading:   true,
		table:     t,
		keys:      defaultLoadBalancersKeyMap(),
	}
	v.Table = &v.table
	return v
}

// Init kicks off the initial load.
func (v *LoadBalancersView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
	if v.client == nil {
		return tea.Batch(v.spinner.Tick, v.initClient())
	}
	return tea.Batch(v.spinner.Tick, v.load())
}

func (v *LoadBalancersView) initClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewComputeClient(gocontext.Background())
		if err != nil {
			return loadBalancersErrorMsg{err: err}
		}
		return loadBalancersClientReadyMsg{client: client}
	}
}

func (v *LoadBalancersView) load() tea.Cmd {
	return func() tea.Msg {
		if v.client == nil {
			return loadBalancersErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		rules, err := v.client.ListForwardingRules(gocontext.Background(), v.projectID)
		if err != nil {
			return loadBalancersErrorMsg{err: err}
		}
		return loadBalancersLoadedMsg{rules: rules}
	}
}

// SetSize records dimensions and resizes the table.
func (v *LoadBalancersView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.table.SetSize(width, height-2)
}

// SetContext updates the view with shared program context.
func (v *LoadBalancersView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.SetSize(ctx.ContentWidth, ctx.ContentHeight)
}

// GetComputeClient exposes the client for cross-view reuse in detail views.
func (v *LoadBalancersView) GetComputeClient() *gcp.ComputeClient {
	return v.client
}

// HasTextInputFocused delegates to the table's filter input.
func (v *LoadBalancersView) HasTextInputFocused() bool {
	return v.table.HasTextInputFocused()
}

// Update routes messages.
func (v *LoadBalancersView) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case loadBalancersClientReadyMsg:
		v.client = m.client
		return v.load()

	case loadBalancersLoadedMsg:
		v.loading = false
		v.err = nil
		v.rules = m.rules
		v.refreshTable()
		return nil

	case loadBalancersErrorMsg:
		v.loading = false
		v.err = m.err
		return nil

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case table.RowDoubleClickedMsg:
		if rule := v.findRule(m.RowID); rule != nil {
			selected := *rule
			return func() tea.Msg {
				return LoadBalancerSelectedMsg{
					SelfLink: selected.SelfLink,
					Scope:    selected.Scope,
					Name:     selected.Name,
				}
			}
		}
		return nil

	case tea.KeyMsg:
		// Let the table handle filter / sort menu input first.
		if v.table.IsSortMenuOpen() || v.table.IsFiltering() {
			var cmd tea.Cmd
			v.table, cmd = v.table.Update(msg)
			return cmd
		}

		switch {
		case key.Matches(m, v.keys.Select):
			row := v.table.SelectedRow()
			if row == nil {
				return nil
			}
			rule := v.findRule(row.ID)
			if rule == nil {
				return nil
			}
			selected := *rule
			return func() tea.Msg {
				return LoadBalancerSelectedMsg{
					SelfLink: selected.SelfLink,
					Scope:    selected.Scope,
					Name:     selected.Name,
				}
			}
		case key.Matches(m, v.keys.Refresh):
			v.loading = true
			v.err = nil
			return tea.Batch(v.spinner.Tick, v.load())
		}
	}

	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

// View renders the view.
func (v *LoadBalancersView) View() string {
	if v.loading {
		return renderLoading(v.spinner, "Loading load balancers...")
	}
	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}
	if len(v.rules) == 0 {
		return "\n  No load balancers found in this project.\n  Press 'esc' to go back."
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • S: sort • /: filter • r: refresh • esc: back")
	return v.table.View() + help
}

func (v *LoadBalancersView) refreshTable() {
	rows := make([]table.Row, 0, len(v.rules))
	for i := range v.rules {
		r := &v.rules[i]
		ports := r.PortRange
		if ports == "" && len(r.Ports) > 0 {
			ports = strings.Join(r.Ports, ",")
		}
		rows = append(rows, table.Row{
			ID:          r.SelfLink,
			Data:        []string{r.Name, r.Type, r.Scope, r.IPAddress, ports, shortNameURL(r.Target)},
			FilterValue: r.Name + " " + r.Type + " " + r.Scope + " " + r.IPAddress + " " + ports,
		})
	}
	v.table.SetRows(rows)
}

func (v *LoadBalancersView) findRule(selfLink string) *gcp.ForwardingRule {
	for i := range v.rules {
		if v.rules[i].SelfLink == selfLink {
			return &v.rules[i]
		}
	}
	return nil
}

// Internal messages.
type loadBalancersClientReadyMsg struct {
	client *gcp.ComputeClient
}

type loadBalancersLoadedMsg struct {
	rules []gcp.ForwardingRule
}

type loadBalancersErrorMsg struct {
	err error
}
