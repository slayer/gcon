package views

import (
	gocontext "context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/tabs"
	"github.com/slayer/gcon/internal/ui/context"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
)

// LoadBalancerDetailsView is the multi-tab inspector for a single forwarding rule.
type LoadBalancerDetailsView struct {
	projectID string
	scope     string
	name      string
	client    *gcp.ComputeClient

	tabs    *tabs.Tabs
	spinner spinner.Model
	width   int
	height  int

	rule     *gcp.ForwardingRule
	proxy    *gcp.TargetProxy
	urlMap   *gcp.URLMap
	backends []gcp.BackendService
	checks   []gcp.HealthCheck

	allFwdRules []gcp.ForwardingRule
	allProxies  []gcp.TargetProxy
	allURLMaps  []gcp.URLMap
	allBackends []gcp.BackendService

	fetchState fetchState
	err        error

	cascade           *Cascade
	showDeleteConfirm bool
	confirmInput      string
	deleting          bool
	deleteErrs        map[string]error

	keys loadBalancerDetailsKeyMap
}

type fetchState struct {
	fwdLoaded           bool
	proxyLoaded         bool
	urlMapLoaded        bool
	backendsLoaded      bool
	checksLoaded        bool
	sharingChecksLoaded bool
}

type loadBalancerDetailsKeyMap struct {
	Refresh key.Binding
	Delete  key.Binding
	Back    key.Binding
}

func defaultLoadBalancerDetailsKeyMap() loadBalancerDetailsKeyMap {
	return loadBalancerDetailsKeyMap{
		Refresh: key.NewBinding(key.WithKeys("r")),
		Delete:  key.NewBinding(key.WithKeys("D")),
		Back:    key.NewBinding(key.WithKeys("esc")),
	}
}

// NewLoadBalancerDetailsView constructs the view.
func NewLoadBalancerDetailsView(projectID, scope, name string, client *gcp.ComputeClient) *LoadBalancerDetailsView {
	t := tabs.New([]tabs.Tab{
		{ID: "overview", Label: "Overview"},
		{ID: "routing", Label: "Routing"},
		{ID: "backends", Label: "Backends"},
	})
	return &LoadBalancerDetailsView{
		projectID: projectID,
		scope:     scope,
		name:      name,
		client:    client,
		tabs:      t,
		spinner:   components.NewGCPSpinner(),
		keys:      defaultLoadBalancerDetailsKeyMap(),
	}
}

// Init begins the parallel fetch.
func (v *LoadBalancerDetailsView) Init() tea.Cmd {
	v.fetchState = fetchState{}
	v.err = nil
	v.cascade = nil
	v.showDeleteConfirm = false
	v.deleteErrs = nil
	v.deleting = false
	if v.client == nil {
		return tea.Batch(v.spinner.Tick, v.initClient())
	}
	return tea.Batch(v.spinner.Tick, v.fetchFwd(), v.fetchSharingInventory())
}

func (v *LoadBalancerDetailsView) initClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewComputeClient(gocontext.Background())
		if err != nil {
			return lbErrorMsg{err: err}
		}
		return lbClientReadyMsg{client: client}
	}
}

// SetSize records dimensions.
func (v *LoadBalancerDetailsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.tabs.SetSize(width)
}

// SetContext mirrors other views.
func (v *LoadBalancerDetailsView) SetContext(ctx *context.ProgramContext) {
	v.SetSize(ctx.ContentWidth, ctx.ContentHeight)
}

// HasTextInputFocused returns true when the delete-confirm input is active so
// global hotkeys (q, etc.) don't fire while the user is typing the LB name.
func (v *LoadBalancerDetailsView) HasTextInputFocused() bool {
	return v.showDeleteConfirm
}

// IsMenuOpen routes Esc to the confirm dialog instead of navigating back.
func (v *LoadBalancerDetailsView) IsMenuOpen() bool {
	return v.showDeleteConfirm
}

// GetComputeClient exposes the client for cross-view reuse.
func (v *LoadBalancerDetailsView) GetComputeClient() *gcp.ComputeClient {
	return v.client
}

// Name returns the forwarding-rule name for breadcrumb rendering.
func (v *LoadBalancerDetailsView) Name() string {
	return v.name
}

// Update routes messages.
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern requires centralized message handling
func (v *LoadBalancerDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case lbClientReadyMsg:
		v.client = m.client
		return tea.Batch(v.fetchFwd(), v.fetchSharingInventory())
	case lbFwdLoadedMsg:
		v.rule = m.rule
		v.fetchState.fwdLoaded = true
		return v.fetchChainCmds()
	case lbProxyLoadedMsg:
		v.proxy = m.proxy
		v.fetchState.proxyLoaded = true
		if m.proxy != nil && m.proxy.URLMap != "" {
			return v.fetchURLMap(m.proxy.URLMap)
		}
		if m.proxy != nil && m.proxy.Service != "" {
			return v.fetchBackends([]string{m.proxy.Service})
		}
		return nil
	case lbURLMapLoadedMsg:
		v.urlMap = m.urlMap
		v.fetchState.urlMapLoaded = true
		if m.urlMap == nil {
			return nil
		}
		return v.fetchBackends(collectBackendURLs(*m.urlMap))
	case lbBackendsLoadedMsg:
		v.backends = m.services
		v.fetchState.backendsLoaded = true
		seen := map[string]struct{}{}
		hcs := []string{}
		for i := range m.services {
			for _, h := range m.services[i].HealthChecks {
				if _, ok := seen[h]; ok {
					continue
				}
				seen[h] = struct{}{}
				hcs = append(hcs, h)
			}
		}
		if len(hcs) == 0 {
			v.fetchState.checksLoaded = true
			return nil
		}
		return v.fetchHealthChecks(hcs)
	case lbHealthChecksLoadedMsg:
		v.checks = m.checks
		v.fetchState.checksLoaded = true
		return nil
	case lbSharingLoadedMsg:
		v.allFwdRules = m.fwdRules
		v.allProxies = m.proxies
		v.allURLMaps = m.urlMaps
		v.allBackends = m.backends
		v.fetchState.sharingChecksLoaded = true
		return nil
	case lbErrorMsg:
		v.err = m.err
		return nil

	case spinner.TickMsg:
		if !v.fetchState.fwdLoaded {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case tea.KeyMsg:
		if v.showDeleteConfirm {
			return v.handleConfirmKey(m)
		}
		switch {
		case key.Matches(m, v.keys.Delete):
			if v.rule == nil || !v.fetchState.sharingChecksLoaded {
				return nil
			}
			c := ComputeCascade(*v.rule, v.allFwdRules, v.allProxies, v.allURLMaps, v.allBackends)
			v.cascade = &c
			v.showDeleteConfirm = true
			v.confirmInput = ""
			return nil
		case key.Matches(m, v.keys.Refresh):
			return v.Init()
		}
		v.tabs.Update(m)
		return nil
	}
	return nil
}

func (v *LoadBalancerDetailsView) handleConfirmKey(m tea.KeyMsg) tea.Cmd {
	switch m.String() {
	case "esc":
		v.showDeleteConfirm = false
		v.cascade = nil
		v.confirmInput = ""
		return nil
	case "enter":
		if v.confirmInput == v.name && v.cascade != nil {
			v.deleting = true
			v.showDeleteConfirm = false
			cascade := *v.cascade
			return func() tea.Msg {
				return LoadBalancerDeleteRequestMsg{Cascade: cascade}
			}
		}
	case "backspace":
		if len(v.confirmInput) > 0 {
			v.confirmInput = v.confirmInput[:len(v.confirmInput)-1]
		}
	default:
		if len(m.Runes) > 0 {
			v.confirmInput += string(m.Runes)
		}
	}
	return nil
}

// View renders the view.
func (v *LoadBalancerDetailsView) View() string {
	if v.err != nil {
		return components.RenderError(v.err)
	}
	if !v.fetchState.fwdLoaded {
		return renderLoading(v.spinner, "Loading load balancer...")
	}

	var b strings.Builder
	b.WriteString(v.tabs.View())
	b.WriteString("\n\n")

	switch v.tabs.ActiveTab().ID {
	case "overview":
		b.WriteString(v.renderOverview())
	case "routing":
		b.WriteString(v.renderRouting())
	case "backends":
		b.WriteString(v.renderBackends())
	}

	if v.deleting {
		b.WriteString("\n\nDeleting load balancer...\n")
	}

	if len(v.deleteErrs) > 0 {
		b.WriteString("\n\n" + v.renderDeleteErrors())
	}

	if v.showDeleteConfirm && v.cascade != nil {
		b.WriteString("\n\n" + v.renderConfirmDialog())
	}

	// Match the left padding other detail views use (e.g. firewall, network,
	// instance details all prefix tabs with two spaces and apply Padding(0, 2)
	// to their viewports).
	return lipgloss.NewStyle().Padding(0, 2).Render(b.String())
}

// renderDeleteErrors lists each resource whose deletion failed, identified by
// its self-link path tail so the user can map failures back to the cascade
// preview.
func (v *LoadBalancerDetailsView) renderDeleteErrors() string {
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Bold(true)
	urls := make([]string, 0, len(v.deleteErrs))
	for u := range v.deleteErrs {
		urls = append(urls, u)
	}
	sort.Strings(urls)
	var b strings.Builder
	b.WriteString(red.Render(fmt.Sprintf("Delete cascade had %d failure(s):", len(v.deleteErrs))))
	b.WriteString("\n")
	for _, u := range urls {
		label := u
		if u != "__client__" {
			label = shortNameURL(u)
		}
		b.WriteString(fmt.Sprintf("  %s: %v\n", label, v.deleteErrs[u]))
	}
	return b.String()
}

func (v *LoadBalancerDetailsView) renderOverview() string {
	r := v.rule
	if r == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Name:    %s\n", r.Name))
	b.WriteString(fmt.Sprintf("Type:    %s\n", r.Type))
	b.WriteString(fmt.Sprintf("Scope:   %s\n", r.Scope))
	b.WriteString(fmt.Sprintf("IP:      %s\n", r.IPAddress))
	ports := r.PortRange
	if ports == "" {
		ports = strings.Join(r.Ports, ",")
	}
	b.WriteString(fmt.Sprintf("Ports:   %s\n", ports))
	b.WriteString(fmt.Sprintf("Scheme:  %s\n", r.LoadBalancingScheme))
	if r.Network != "" {
		b.WriteString(fmt.Sprintf("Network: %s\n", r.Network))
	}
	if r.Subnetwork != "" {
		b.WriteString(fmt.Sprintf("Subnet:  %s\n", r.Subnetwork))
	}
	if v.proxy != nil {
		b.WriteString(fmt.Sprintf("\nProxy:   %s (%s)\n", v.proxy.Name, v.proxy.Kind))
		if v.proxy.SSLPolicy != "" {
			b.WriteString(fmt.Sprintf("  SSL policy: %s\n", v.proxy.SSLPolicy))
		}
		if len(v.proxy.SSLCertificates) > 0 {
			names := make([]string, 0, len(v.proxy.SSLCertificates))
			for _, certURL := range v.proxy.SSLCertificates {
				names = append(names, shortNameURL(certURL))
			}
			b.WriteString(fmt.Sprintf("  Certificates: %s\n", strings.Join(names, ", ")))
		}
	}
	return b.String()
}

func (v *LoadBalancerDetailsView) renderRouting() string {
	if v.urlMap == nil {
		return "(no URL map — Network LB or pending)"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Default service: %s\n\n", shortNameURL(v.urlMap.DefaultService)))
	for _, hr := range v.urlMap.HostRules {
		b.WriteString(fmt.Sprintf("Hosts: %s → matcher: %s\n", strings.Join(hr.Hosts, ", "), hr.PathMatcher))
	}
	for _, pm := range v.urlMap.PathMatchers {
		b.WriteString(fmt.Sprintf("\nMatcher %s (default → %s)\n", pm.Name, shortNameURL(pm.DefaultService)))
		for _, pr := range pm.PathRules {
			b.WriteString(fmt.Sprintf("  %s → %s\n", strings.Join(pr.Paths, ", "), shortNameURL(pr.Service)))
		}
	}
	return b.String()
}

func (v *LoadBalancerDetailsView) renderBackends() string {
	if !v.fetchState.backendsLoaded {
		return "Loading backends..."
	}
	if len(v.backends) == 0 {
		return "(no backends)"
	}
	var b strings.Builder
	for i := range v.backends {
		bs := &v.backends[i]
		b.WriteString(fmt.Sprintf("Backend service: %s\n", bs.Name))
		b.WriteString(fmt.Sprintf("  Protocol: %s  Timeout: %ds  Affinity: %s\n", bs.Protocol, bs.TimeoutSec, bs.SessionAffinity))
		for _, be := range bs.Backends {
			b.WriteString(fmt.Sprintf("    Group: %s  Mode: %s  Cap: %.2f\n", shortNameURL(be.Group), be.BalancingMode, be.CapacityScaler))
		}
		for _, hcURL := range bs.HealthChecks {
			b.WriteString(fmt.Sprintf("    Health check: %s\n", shortNameURL(hcURL)))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (v *LoadBalancerDetailsView) renderConfirmDialog() string {
	if v.cascade == nil {
		return ""
	}
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Bold(true)
	var b strings.Builder
	b.WriteString(red.Render(fmt.Sprintf("Delete load balancer: %s", v.name)))
	b.WriteString("\n\nWill delete:\n")
	for _, it := range v.cascade.Delete {
		b.WriteString(fmt.Sprintf("  %s: %s (%s)\n", it.Kind, it.Name, it.Scope))
	}
	if len(v.cascade.Keep) > 0 {
		b.WriteString("\nWill keep (still in use):\n")
		for _, k := range v.cascade.Keep {
			b.WriteString(fmt.Sprintf("  %s: %s — %s\n", k.Kind, k.Name, strings.Join(k.KeptBecause, ", ")))
		}
	}
	b.WriteString(fmt.Sprintf("\nType the LB name to confirm: %s_\n", v.confirmInput))
	b.WriteString("\n[Enter] Delete  [Esc] Cancel\n")
	return b.String()
}

// SetDeleteResult is called by the app handler after the cascade executor
// finishes. Per-resource failures are stored on the view and rendered inline
// inside the details body so the user keeps the cascade context. We do not
// touch v.err, which is reserved for fetch failures that fully replace the
// view.
func (v *LoadBalancerDetailsView) SetDeleteResult(errs map[string]error) {
	v.deleting = false
	v.deleteErrs = errs
}

// --- async fetch helpers ---

func (v *LoadBalancerDetailsView) fetchFwd() tea.Cmd {
	if v.client == nil {
		return func() tea.Msg { return lbErrorMsg{err: uierrors.ErrClientNotInitialized} }
	}
	return func() tea.Msg {
		fr, err := v.client.GetForwardingRule(gocontext.Background(), v.projectID, v.scope, v.name)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		return lbFwdLoadedMsg{rule: fr}
	}
}

func (v *LoadBalancerDetailsView) fetchChainCmds() tea.Cmd {
	if v.rule == nil || v.rule.Target == "" {
		return nil
	}
	kind := targetKindFromURL(v.rule.Target)
	switch kind {
	case "targetHttpProxies", "targetHttpsProxies", "targetTcpProxies", "targetSslProxies":
		return v.fetchProxy(v.rule.Target)
	case "backendServices":
		return v.fetchBackends([]string{v.rule.Target})
	}
	return nil
}

func (v *LoadBalancerDetailsView) fetchProxy(url string) tea.Cmd {
	if v.client == nil {
		return func() tea.Msg { return lbErrorMsg{err: uierrors.ErrClientNotInitialized} }
	}
	scope := scopeFromURL(url)
	return func() tea.Msg {
		tp, err := v.client.GetTargetProxy(gocontext.Background(), v.projectID, scope, url)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		return lbProxyLoadedMsg{proxy: tp}
	}
}

func (v *LoadBalancerDetailsView) fetchURLMap(url string) tea.Cmd {
	if v.client == nil {
		return nil
	}
	scope := scopeFromURL(url)
	return func() tea.Msg {
		um, err := v.client.GetURLMap(gocontext.Background(), v.projectID, scope, url)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		return lbURLMapLoadedMsg{urlMap: um}
	}
}

func (v *LoadBalancerDetailsView) fetchBackends(urls []string) tea.Cmd {
	if v.client == nil || len(urls) == 0 {
		return nil
	}
	return func() tea.Msg {
		var services []gcp.BackendService
		for _, u := range urls {
			bs, err := v.client.GetBackendService(gocontext.Background(), v.projectID, scopeFromURL(u), u)
			if err != nil {
				return lbErrorMsg{err: err}
			}
			services = append(services, *bs)
		}
		return lbBackendsLoadedMsg{services: services}
	}
}

func (v *LoadBalancerDetailsView) fetchHealthChecks(urls []string) tea.Cmd {
	if v.client == nil || len(urls) == 0 {
		return nil
	}
	return func() tea.Msg {
		var checks []gcp.HealthCheck
		for _, u := range urls {
			hc, err := v.client.GetHealthCheck(gocontext.Background(), v.projectID, scopeFromURL(u), u)
			if err != nil {
				return lbErrorMsg{err: err}
			}
			checks = append(checks, *hc)
		}
		return lbHealthChecksLoadedMsg{checks: checks}
	}
}

func (v *LoadBalancerDetailsView) fetchSharingInventory() tea.Cmd {
	if v.client == nil {
		return nil
	}
	return func() tea.Msg {
		ctx := gocontext.Background()
		fwds, err := v.client.ListForwardingRules(ctx, v.projectID)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		proxies, err := v.client.ListAllProxies(ctx, v.projectID)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		urlMaps, err := v.client.ListAllURLMaps(ctx, v.projectID)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		backends, err := v.client.ListAllBackendServices(ctx, v.projectID)
		if err != nil {
			return lbErrorMsg{err: err}
		}
		return lbSharingLoadedMsg{fwdRules: fwds, proxies: proxies, urlMaps: urlMaps, backends: backends}
	}
}

func collectBackendURLs(um gcp.URLMap) []string {
	seen := map[string]struct{}{}
	out := []string{}
	add := func(s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(um.DefaultService)
	for _, pm := range um.PathMatchers {
		add(pm.DefaultService)
		for _, pr := range pm.PathRules {
			add(pr.Service)
		}
	}
	return out
}

// Internal messages.
type lbClientReadyMsg struct{ client *gcp.ComputeClient }
type lbFwdLoadedMsg struct{ rule *gcp.ForwardingRule }
type lbProxyLoadedMsg struct{ proxy *gcp.TargetProxy }
type lbURLMapLoadedMsg struct{ urlMap *gcp.URLMap }
type lbBackendsLoadedMsg struct{ services []gcp.BackendService }
type lbHealthChecksLoadedMsg struct{ checks []gcp.HealthCheck }
type lbSharingLoadedMsg struct {
	fwdRules []gcp.ForwardingRule
	proxies  []gcp.TargetProxy
	urlMaps  []gcp.URLMap
	backends []gcp.BackendService
}
type lbErrorMsg struct{ err error }
