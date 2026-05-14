package views

import (
	gocontext "context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
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
	gcpClient *gcp.Client

	tabs         *tabs.Tabs
	spinner      spinner.Model
	viewport     viewport.Model
	viewportSize bool // true once SetSize has been called
	width   int
	height  int

	rule     *gcp.ForwardingRule
	proxy    *gcp.TargetProxy
	urlMap   *gcp.URLMap
	backends []gcp.BackendService
	checks   []gcp.HealthCheck

	// Backend group health (phase 2).
	groupHealth   map[string]groupHealthState // keyed on Backend.Group URL
	groupFocus    int                         // index into the flat list of group rows; -1 = no focus
	groupExpanded map[string]bool             // group URL -> expanded?

	allFwdRules []gcp.ForwardingRule
	allProxies  []gcp.TargetProxy
	allURLMaps  []gcp.URLMap
	allBackends []gcp.BackendService

	fetchState fetchState
	err        error
	// sharingErr is reserved for sharing-inventory fetch failures. Unlike
	// v.err it does NOT replace the view — it disables delete and shows an
	// inline warning so the user can still inspect the LB.
	sharingErr error

	observability *loadBalancerObservability

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
func NewLoadBalancerDetailsView(projectID, scope, name string, client *gcp.ComputeClient, gcpClient *gcp.Client) *LoadBalancerDetailsView {
	t := tabs.New([]tabs.Tab{
		{ID: "overview", Label: "Overview"},
		{ID: "routing", Label: "Routing"},
		{ID: "backends", Label: "Backends"},
		{ID: "observability", Label: "Observability"},
	})
	return &LoadBalancerDetailsView{
		projectID:     projectID,
		scope:         scope,
		name:          name,
		client:        client,
		gcpClient:     gcpClient,
		tabs:          t,
		spinner:       components.NewGCPSpinner(),
		keys:          defaultLoadBalancerDetailsKeyMap(),
		groupHealth:   map[string]groupHealthState{},
		groupExpanded: map[string]bool{},
		groupFocus:    -1,
	}
}

// Init begins the parallel fetch.
func (v *LoadBalancerDetailsView) Init() tea.Cmd {
	v.fetchState = fetchState{}
	v.err = nil
	v.sharingErr = nil
	v.cascade = nil
	v.showDeleteConfirm = false
	v.deleteErrs = nil
	v.deleting = false
	v.groupHealth = map[string]groupHealthState{}
	v.groupExpanded = map[string]bool{}
	v.groupFocus = -1
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
	// Reserve rows for the tab strip (1), blank line under tabs (1), the
	// outer Padding(0, 2) — top/bottom — and a buffer for inline error /
	// confirm-dialog chrome that occasionally renders below the body.
	vpW := max(1, width-4)
	vpH := max(1, height-4)
	if !v.viewportSize {
		v.viewport = viewport.New(vpW, vpH)
		v.viewportSize = true
	} else {
		v.viewport.Width = vpW
		v.viewport.Height = vpH
	}
	if v.observability != nil {
		v.observability.width = vpW
		v.observability.resizeCharts()
	}
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
		chainCmd := v.fetchChainCmds()
		// If the user landed on the Observability tab before the rule
		// loaded, the lazy-init was gated off — kick it now that we know
		// the LB type.
		if v.tabs.ActiveTab().ID == "observability" {
			if obsCmd := v.ensureObservability(); obsCmd != nil {
				if chainCmd != nil {
					return tea.Batch(chainCmd, obsCmd)
				}
				return obsCmd
			}
		}
		return chainCmd
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
		healthCmd := v.fetchAllBackendHealth()
		if len(hcs) == 0 {
			v.fetchState.checksLoaded = true
			return healthCmd
		}
		return tea.Batch(v.fetchHealthChecks(hcs), healthCmd)
	case lbHealthChecksLoadedMsg:
		v.checks = m.checks
		v.fetchState.checksLoaded = true
		return nil
	case lbGroupHealthLoadedMsg:
		v.groupHealth[m.groupURL] = groupHealthState{
			phase:    groupHealthOK,
			statuses: m.statuses,
		}
		return nil
	case lbGroupHealthErrorMsg:
		v.groupHealth[m.groupURL] = groupHealthState{
			phase: groupHealthErrored,
			err:   m.err,
		}
		return nil
	case lbGroupSkippedMsg:
		v.groupHealth[m.groupURL] = groupHealthState{
			phase:  groupHealthSkipped,
			reason: m.reason,
		}
		return nil
	case lbHealthRefreshMsg:
		return v.fetchAllBackendHealth()
	case lbSharingLoadedMsg:
		v.allFwdRules = m.fwdRules
		v.allProxies = m.proxies
		v.allURLMaps = m.urlMaps
		v.allBackends = m.backends
		v.fetchState.sharingChecksLoaded = true
		v.sharingErr = nil
		return nil
	case lbSharingErrorMsg:
		v.sharingErr = m.err
		return nil
	case lbErrorMsg:
		v.err = m.err
		return nil

	case tabs.TabChangedMsg:
		// Reset scroll when switching tabs so each tab starts at the top.
		v.viewport.GotoTop()
		if v.tabs.ActiveTab().ID == "observability" {
			// If the rule hasn't loaded yet, the sub-view will be created
			// later by the lbFwdLoadedMsg handler. Until then the
			// placeholder/loading row in renderObservability covers the UI.
			if obsCmd := v.ensureObservability(); obsCmd != nil {
				return obsCmd
			}
			if v.observability == nil {
				return nil
			}
			return v.observability.StartAutoRefresh()
		}
		if v.observability != nil {
			v.observability.StopAutoRefresh()
		}
		return nil

	case lbMetricsLoadedMsg, lbMetricsErrorMsg, lbObsTickMsg:
		if v.observability != nil {
			return v.observability.Update(m)
		}
		return nil

	case spinner.TickMsg:
		var cmds []tea.Cmd
		if !v.fetchState.fwdLoaded {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if v.observability != nil && v.observability.metricsLoading {
			if cmd := v.observability.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)

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
		if v.tabs.ActiveTab().ID == "observability" && v.observability != nil {
			if cmd, handled := v.observability.handleKey(m); handled {
				return cmd
			}
		}
		if v.tabs.ActiveTab().ID == "backends" {
			if cmd, handled := v.handleBackendsKey(m); handled {
				return cmd
			}
		}
		if isViewportScrollKey(m) {
			var cmd tea.Cmd
			v.viewport, cmd = v.viewport.Update(m)
			return cmd
		}
		// tabs.Update returns a tea.Cmd that emits TabChangedMsg; propagate it.
		return v.tabs.Update(m)
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

	var body string
	switch v.tabs.ActiveTab().ID {
	case "overview":
		body = v.renderOverview()
	case "routing":
		body = v.renderRouting()
	case "backends":
		body = v.renderBackends()
	case "observability":
		body = v.renderObservability()
	}
	if v.viewportSize {
		v.viewport.SetContent(body)
		b.WriteString(v.viewport.View())
	} else {
		b.WriteString(body)
	}

	if v.deleting {
		b.WriteString("\n\nDeleting load balancer...\n")
	}

	if v.sharingErr != nil {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04"))
		b.WriteString("\n\n")
		b.WriteString(warnStyle.Render(fmt.Sprintf("⚠ Could not load sharing inventory: %v", v.sharingErr)))
		b.WriteString("\n")
		b.WriteString(warnStyle.Render("  Delete is disabled (would not be safe without dependency graph)."))
		b.WriteString("\n")
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

// ensureObservability lazy-creates the observability sub-view when the
// active tab is "observability" and the forwarding rule is metric-capable.
// Returns the Init+StartAutoRefresh batch on first creation, nil otherwise
// (rule still loading, or non-capable LB type). Callers in the tab-change
// path and the rule-loaded path both rely on this to handle the race where
// the user switches to the tab before the rule arrives.
func (v *LoadBalancerDetailsView) ensureObservability() tea.Cmd {
	if v.observability != nil {
		return nil
	}
	resourceType := resourceTypeForLB(v.rule)
	if resourceType == "" {
		return nil
	}
	v.observability = newLoadBalancerObservability(v.projectID, v.name, resourceType, v.gcpClient)
	v.observability.width = max(1, v.width-4)
	v.observability.resizeCharts()
	return tea.Batch(v.observability.Init(), v.observability.StartAutoRefresh())
}

func (v *LoadBalancerDetailsView) renderObservability() string {
	// View() gates the whole tab on fwdLoaded, but if the upstream fetch
	// returns a nil rule with no error, v.rule can still be nil here.
	// Show loading rather than the "not supported" placeholder so the
	// user isn't misled about the LB's capabilities.
	if !v.fetchState.fwdLoaded || v.rule == nil {
		return renderLoading(v.spinner, "Loading observability...")
	}
	if !isHTTPSObservabilityCapable(v.rule) {
		return v.renderObservabilityPlaceholder()
	}
	if v.observability == nil {
		return renderLoading(v.spinner, "Loading observability...")
	}
	return v.observability.View()
}

func (v *LoadBalancerDetailsView) renderObservabilityPlaceholder() string {
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	kind := "this LB type"
	if v.rule != nil && v.rule.Type != "" {
		kind = v.rule.Type
	}
	var b strings.Builder
	b.WriteString("Observability\n")
	b.WriteString(strings.Repeat("─", 60))
	b.WriteString("\n\n")
	b.WriteString(muted.Render(fmt.Sprintf("  Metrics for %s are not yet supported in gcon.", kind)))
	b.WriteString("\n")
	b.WriteString(muted.Render("  The l3/* metric family for passthrough/proxy Network LBs is on the roadmap."))
	b.WriteString("\n\n")
	b.WriteString(muted.Render("  View metrics in the GCP console:"))
	b.WriteString("\n")
	b.WriteString(muted.Render("    https://console.cloud.google.com/net-services/loadbalancing"))
	b.WriteString("\n")
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
		// Sharing inventory is only needed for the delete cascade. Errors here
		// must NOT replace the details view — fall through to lbSharingErrorMsg
		// so the user can still browse Overview / Routing / Backends.
		fwds, err := v.client.ListForwardingRules(ctx, v.projectID)
		if err != nil {
			return lbSharingErrorMsg{err: err}
		}
		proxies, err := v.client.ListAllProxies(ctx, v.projectID)
		if err != nil {
			return lbSharingErrorMsg{err: err}
		}
		urlMaps, err := v.client.ListAllURLMaps(ctx, v.projectID)
		if err != nil {
			return lbSharingErrorMsg{err: err}
		}
		backends, err := v.client.ListAllBackendServices(ctx, v.projectID)
		if err != nil {
			return lbSharingErrorMsg{err: err}
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

// isHTTPSObservabilityCapable returns true when the forwarding rule is an
// HTTP / HTTPS / internal HTTPS LB, which are the types covered by the
// loadbalancing.googleapis.com/https/* metric family.
func isHTTPSObservabilityCapable(r *gcp.ForwardingRule) bool {
	return resourceTypeForLB(r) != ""
}

// isViewportScrollKey identifies keys that should scroll the tab body's
// viewport when no tab-specific handler has consumed them. PgUp/PgDn and
// Home/End work on every tab; j/k/up/down work only when the Backends
// tab's group-focus handler hasn't already taken them.
func isViewportScrollKey(m tea.KeyMsg) bool {
	switch m.String() {
	case "j", "k", "up", "down", "pgup", "pgdown", "home", "end":
		return true
	}
	return false
}

// resourceTypeForLB maps a forwarding rule's human-readable type label to
// the Cloud Monitoring resource type that hosts its metrics. Returns "" if
// the LB isn't supported by the loadbalancing.googleapis.com/https/*
// metric family (Network LBs, TCP/SSL proxies, legacy target pools).
//
// GCP's filter language disallows OR between resource.type values, so the
// caller must pick exactly one per fetch — this is the mapping function.
func resourceTypeForLB(r *gcp.ForwardingRule) string {
	if r == nil {
		return ""
	}
	switch r.Type {
	case "HTTPS (external)", "HTTP (external)":
		return "https_lb_rule"
	case "HTTPS (internal)", "HTTP (internal)":
		return "internal_http_lb_rule"
	}
	return ""
}
