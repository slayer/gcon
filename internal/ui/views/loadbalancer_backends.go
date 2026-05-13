package views

import (
	gocontext "context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/slayer/gcon/internal/gcp"
)

// groupHealthPhase is the lifecycle of one backend group's health-fetch
// result. The zero value is groupHealthLoading so a freshly-seeded entry
// in the groupHealth map renders as "loading…" without explicit init.
type groupHealthPhase int

const (
	groupHealthLoading groupHealthPhase = iota
	groupHealthOK
	groupHealthErrored
	groupHealthSkipped
)

type groupHealthState struct {
	phase    groupHealthPhase
	statuses []gcp.InstanceHealth
	err      error
	reason   string // populated when phase == groupHealthSkipped
}

type backendGroupKind int

const (
	backendGroupUnknown backendGroupKind = iota
	backendGroupInstanceGroup
	backendGroupNEG
)

func (v *LoadBalancerDetailsView) renderBackends() string {
	if !v.fetchState.backendsLoaded {
		return "Loading backends..."
	}
	if len(v.backends) == 0 {
		return "(no backends)"
	}
	urls := v.flatGroupURLs()
	focusedURL := ""
	if v.groupFocus >= 0 && v.groupFocus < len(urls) {
		focusedURL = urls[v.groupFocus]
	}
	var b strings.Builder
	for i := range v.backends {
		bs := &v.backends[i]
		b.WriteString(fmt.Sprintf("Backend service: %s\n", bs.Name))
		b.WriteString(fmt.Sprintf("  Protocol: %s  Timeout: %ds  Affinity: %s\n", bs.Protocol, bs.TimeoutSec, bs.SessionAffinity))
		for _, be := range bs.Backends {
			cursor := "   "
			if be.Group == focusedURL {
				cursor = " ▸ "
			}
			b.WriteString(fmt.Sprintf("  %sGroup: %s  Mode: %s  Cap: %.2f  %s\n",
				cursor, shortNameURL(be.Group), be.BalancingMode, be.CapacityScaler, v.renderHealthBadge(be.Group)))
			if v.groupExpanded[be.Group] {
				b.WriteString(v.renderHealthExpansion(be.Group))
			}
		}
		for _, hcURL := range bs.HealthChecks {
			b.WriteString(fmt.Sprintf("    Health check: %s\n", shortNameURL(hcURL)))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderHealthBadge returns the inline summary for one backend group.
// Empty string when no health is known yet (e.g. before fan-out fires).
func (v *LoadBalancerDetailsView) renderHealthBadge(groupURL string) string {
	st, ok := v.groupHealth[groupURL]
	if !ok {
		return ""
	}
	switch st.phase {
	case groupHealthLoading:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Render("◌◌ loading…")
	case groupHealthErrored:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335")).Render(fmt.Sprintf("? error: %v", st.err))
	case groupHealthSkipped:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Render(fmt.Sprintf("(no health — %s)", st.reason))
	case groupHealthOK:
		return v.renderHealthSummary(st.statuses)
	}
	return ""
}

// renderHealthSummary draws the green/red dot row and counts. Up to 5
// per-instance dots inline; beyond that, abbreviate to "N/M healthy".
func (v *LoadBalancerDetailsView) renderHealthSummary(statuses []gcp.InstanceHealth) string {
	total := len(statuses)
	if total == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Render("(no members)")
	}
	healthy := 0
	for _, s := range statuses {
		if s.HealthState == "HEALTHY" {
			healthy++
		}
	}
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))

	if total > 5 {
		return fmt.Sprintf("%s %d/%d healthy", green.Render("●"), healthy, total)
	}
	var dots strings.Builder
	for _, s := range statuses {
		if s.HealthState == "HEALTHY" {
			dots.WriteString(green.Render("●"))
		} else {
			dots.WriteString(red.Render("○"))
		}
	}
	return fmt.Sprintf("%s %d/%d healthy", dots.String(), healthy, total)
}

// renderHealthExpansion draws the per-instance health table for an
// expanded group. Each row shows the instance short name, IP:port,
// state, and (for unhealthy members) the failure reason.
func (v *LoadBalancerDetailsView) renderHealthExpansion(groupURL string) string {
	st, ok := v.groupHealth[groupURL]
	if !ok || st.phase != groupHealthOK {
		return ""
	}
	var b strings.Builder
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#EA4335"))
	gray := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))

	for _, s := range st.statuses {
		dot := gray.Render("○")
		state := s.HealthState
		switch s.HealthState {
		case "HEALTHY":
			dot = green.Render("●")
		case "UNHEALTHY":
			dot = red.Render("●")
		case "DRAINING":
			dot = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBC04")).Render("●")
		}
		addr := s.Instance
		if s.IPAddress != "" {
			if s.Port > 0 {
				addr = fmt.Sprintf("%s (%s:%d)", s.Instance, s.IPAddress, s.Port)
			} else {
				addr = fmt.Sprintf("%s (%s)", s.Instance, s.IPAddress)
			}
		}
		reason := s.FailureReason
		if reason == "" {
			b.WriteString(fmt.Sprintf("        %s %-30s %s\n", dot, addr, state))
		} else {
			b.WriteString(fmt.Sprintf("        %s %-30s %s — %s\n", dot, addr, state, reason))
		}
	}
	return b.String()
}

// flatGroupURLs returns every Backend.Group URL across every backend
// service in display order (deduped). Used to map a groupFocus index to a URL.
func (v *LoadBalancerDetailsView) flatGroupURLs() []string {
	urls := make([]string, 0)
	seen := map[string]struct{}{}
	for i := range v.backends {
		for _, be := range v.backends[i].Backends {
			if be.Group == "" {
				continue
			}
			if _, ok := seen[be.Group]; ok {
				continue
			}
			seen[be.Group] = struct{}{}
			urls = append(urls, be.Group)
		}
	}
	return urls
}

// handleBackendsKey processes j/k navigation and Esc on the Backends tab.
// Returns (cmd, true) when handled, (nil, false) otherwise.
func (v *LoadBalancerDetailsView) handleBackendsKey(m tea.KeyMsg) (tea.Cmd, bool) {
	urls := v.flatGroupURLs()
	if len(urls) == 0 {
		return nil, false
	}
	switch m.String() {
	case "j", "down":
		if v.groupFocus < 0 {
			v.groupFocus = 0
		} else if v.groupFocus < len(urls)-1 {
			v.groupFocus++
		}
		return nil, true
	case "k", "up":
		if v.groupFocus > 0 {
			v.groupFocus--
		}
		return nil, true
	case "enter", "tab":
		if v.groupFocus < 0 || v.groupFocus >= len(urls) {
			return nil, false
		}
		url := urls[v.groupFocus]
		v.groupExpanded[url] = !v.groupExpanded[url]
		return nil, true
	case "esc":
		if v.groupFocus >= 0 {
			v.groupFocus = -1
			return nil, true
		}
	}
	return nil, false
}

// fetchAllBackendHealth returns one tea.Cmd per (backend service, group)
// pair across the freshly-loaded backend services. Each cmd ultimately
// emits one of: lbGroupHealthLoadedMsg, lbGroupHealthErrorMsg,
// lbGroupSkippedMsg. The view's groupHealth map is keyed by Backend.Group
// URL, so duplicate Group URLs (rare — same group referenced by multiple
// backend services) share state.
func (v *LoadBalancerDetailsView) fetchAllBackendHealth() tea.Cmd {
	if v.client == nil {
		return nil
	}
	var cmds []tea.Cmd
	seen := map[string]struct{}{}
	for i := range v.backends {
		bs := &v.backends[i]
		for _, be := range bs.Backends {
			if be.Group == "" {
				continue
			}
			if _, ok := seen[be.Group]; ok {
				continue
			}
			seen[be.Group] = struct{}{}
			v.groupHealth[be.Group] = groupHealthState{phase: groupHealthLoading}
			cmds = append(cmds, v.fetchGroupHealth(bs.Name, bs.Scope, be.Group))
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// fetchGroupHealth dispatches the health resolution for a single group.
// Instance groups call GetBackendHealth directly. NEGs first GET the NEG
// to check for SERVERLESS endpoint type (no per-instance health), then
// either call GetBackendHealth or emit a skip. Unknown URL kinds skip.
func (v *LoadBalancerDetailsView) fetchGroupHealth(backendServiceName, scope, groupURL string) tea.Cmd {
	kind := classifyBackendGroupURL(groupURL)
	switch kind {
	case backendGroupInstanceGroup:
		return v.fetchInstanceGroupHealth(backendServiceName, scope, groupURL)
	case backendGroupNEG:
		return v.fetchNEGHealth(backendServiceName, scope, groupURL)
	default:
		return func() tea.Msg {
			return lbGroupSkippedMsg{groupURL: groupURL, reason: "unsupported backend kind"}
		}
	}
}

func (v *LoadBalancerDetailsView) fetchInstanceGroupHealth(backendServiceName, scope, groupURL string) tea.Cmd {
	return func() tea.Msg {
		statuses, err := v.client.GetBackendHealth(gocontext.Background(), v.projectID, scope, backendServiceName, groupURL)
		if err != nil {
			return lbGroupHealthErrorMsg{groupURL: groupURL, err: err}
		}
		return lbGroupHealthLoadedMsg{groupURL: groupURL, statuses: statuses}
	}
}

func (v *LoadBalancerDetailsView) fetchNEGHealth(backendServiceName, scope, groupURL string) tea.Cmd {
	return func() tea.Msg {
		negScope := groupScope(groupURL)
		neg, err := v.client.GetNetworkEndpointGroup(gocontext.Background(), v.projectID, negScope, groupURL)
		if err != nil {
			return lbGroupHealthErrorMsg{groupURL: groupURL, err: err}
		}
		if neg.NetworkEndpointType == "SERVERLESS" {
			return lbGroupSkippedMsg{groupURL: groupURL, reason: "serverless NEG"}
		}
		statuses, err := v.client.GetBackendHealth(gocontext.Background(), v.projectID, scope, backendServiceName, groupURL)
		if err != nil {
			return lbGroupHealthErrorMsg{groupURL: groupURL, err: err}
		}
		return lbGroupHealthLoadedMsg{groupURL: groupURL, statuses: statuses}
	}
}

// classifyBackendGroupURL inspects a Backend.Group URL and returns a
// coarse kind. Distinguishing serverless from VM-backed NEGs requires
// a separate API GET (see fetchGroupHealth in Task 12).
func classifyBackendGroupURL(url string) backendGroupKind {
	if url == "" {
		return backendGroupUnknown
	}
	switch {
	case strings.Contains(url, "/instanceGroups/"):
		return backendGroupInstanceGroup
	case strings.Contains(url, "/networkEndpointGroups/"):
		return backendGroupNEG
	default:
		return backendGroupUnknown
	}
}

// groupScope extracts the zone-or-region segment from a Backend.Group
// URL. Returns "global" if the URL contains "/global/" instead of a
// zone or region (rare, but possible for global NEGs).
func groupScope(url string) string {
	parts := strings.Split(url, "/")
	for i, p := range parts {
		switch p {
		case "zones", "regions":
			if i+1 < len(parts) {
				return parts[i+1]
			}
		case "global":
			return "global"
		}
	}
	return ""
}
