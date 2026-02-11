package gcp

import (
	"context"
	"sort"
	"strings"

	"google.golang.org/api/compute/v1"
)

// FirewallRule is the list-view summary
type FirewallRule struct {
	Name      string
	ID        uint64
	Direction string // INGRESS or EGRESS
	Action    string // ALLOW or DENY
	Priority  int64
	Protocols string // Summarized: e.g. "tcp:80,443 icmp"
	Network   string // Short name extracted from URL
	Disabled  bool
	CreatedAt string
}

// FirewallRuleDetails is the full detail for the details view
type FirewallRuleDetails struct {
	Name                  string
	ID                    uint64
	Description           string
	Direction             string
	Action                string // "ALLOW" or "DENY" (derived from which of Allowed/Denied is non-empty)
	Priority              int64
	Disabled              bool
	Network               string // Short name extracted from URL
	NetworkURL            string // Full URL for navigation
	Allowed               []FirewallRuleEntry
	Denied                []FirewallRuleEntry
	SourceRanges          []string
	DestinationRanges     []string
	SourceTags            []string
	TargetTags            []string
	SourceServiceAccounts []string
	TargetServiceAccounts []string
	LogEnabled            bool
	LogMetadata           string
	CreatedAt             string
	SelfLink              string
}

// FirewallRuleEntry represents an allow/deny entry with protocol and optional ports
type FirewallRuleEntry struct {
	Protocol string
	Ports    []string
}

// ListFirewallRules retrieves all firewall rules in a project
func (c *ComputeClient) ListFirewallRules(ctx context.Context, projectID string) ([]FirewallRule, error) {
	var rules []FirewallRule

	req := c.service.Firewalls.List(projectID)
	err := req.Pages(ctx, func(page *compute.FirewallList) error {
		for _, fw := range page.Items {
			rules = append(rules, firewallRuleFromAPI(fw))
		}
		return nil
	})
	if err != nil {
		return nil, WrapListError(err, "firewall rules", projectID)
	}

	// Sort by priority (ascending), then name (alphabetical) for consistent display
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority < rules[j].Priority
		}
		return rules[i].Name < rules[j].Name
	})

	return rules, nil
}

// GetFirewallRuleDetails fetches detailed info for a single firewall rule
func (c *ComputeClient) GetFirewallRuleDetails(ctx context.Context, projectID, ruleName string) (*FirewallRuleDetails, error) {
	fw, err := c.service.Firewalls.Get(projectID, ruleName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "firewall rule", ruleName)
	}
	return firewallRuleDetailsFromAPI(fw), nil
}

// DeleteFirewallRule deletes a firewall rule
func (c *ComputeClient) DeleteFirewallRule(ctx context.Context, projectID, ruleName string) error {
	_, err := c.service.Firewalls.Delete(projectID, ruleName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "delete firewall rule", ruleName)
	}
	return nil
}

// SetFirewallRuleDisabled enables or disables a firewall rule.
// ForceSendFields ensures the bool is sent even when false (GCP omits zero-value bools otherwise).
func (c *ComputeClient) SetFirewallRuleDisabled(ctx context.Context, projectID, ruleName string, disabled bool) error {
	_, err := c.service.Firewalls.Patch(projectID, ruleName, &compute.Firewall{
		Disabled:        disabled,
		ForceSendFields: []string{"Disabled"},
	}).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "update firewall rule", ruleName)
	}
	return nil
}

// firewallRuleFromAPI converts a Compute Engine Firewall to our list-view summary
func firewallRuleFromAPI(fw *compute.Firewall) FirewallRule {
	return FirewallRule{
		Name:      fw.Name,
		ID:        fw.Id,
		Direction: fw.Direction,
		Action:    deriveAction(fw),
		Priority:  fw.Priority,
		Protocols: summarizeProtocols(fw.Allowed, fw.Denied),
		Network:   extractNameFromURL(fw.Network),
		Disabled:  fw.Disabled,
		CreatedAt: fw.CreationTimestamp,
	}
}

// firewallRuleDetailsFromAPI converts a Compute Engine Firewall to full details
func firewallRuleDetailsFromAPI(fw *compute.Firewall) *FirewallRuleDetails {
	details := &FirewallRuleDetails{
		Name:                  fw.Name,
		ID:                    fw.Id,
		Description:           fw.Description,
		Direction:             fw.Direction,
		Action:                deriveAction(fw),
		Priority:              fw.Priority,
		Disabled:              fw.Disabled,
		Network:               extractNameFromURL(fw.Network),
		NetworkURL:            fw.Network,
		Allowed:               convertAllowed(fw.Allowed),
		Denied:                convertDenied(fw.Denied),
		SourceRanges:          fw.SourceRanges,
		DestinationRanges:     fw.DestinationRanges,
		SourceTags:            fw.SourceTags,
		TargetTags:            fw.TargetTags,
		SourceServiceAccounts: fw.SourceServiceAccounts,
		TargetServiceAccounts: fw.TargetServiceAccounts,
		CreatedAt:             fw.CreationTimestamp,
		SelfLink:              fw.SelfLink,
	}

	if fw.LogConfig != nil {
		details.LogEnabled = fw.LogConfig.Enable
		details.LogMetadata = fw.LogConfig.Metadata
	}

	return details
}

// summarizeProtocols builds a compact protocol summary like "tcp:80,443 icmp" or "all"
func summarizeProtocols(allowed []*compute.FirewallAllowed, denied []*compute.FirewallDenied) string {
	var parts []string

	for _, a := range allowed {
		parts = append(parts, formatProtocolPorts(a.IPProtocol, a.Ports))
	}
	for _, d := range denied {
		parts = append(parts, formatProtocolPorts(d.IPProtocol, d.Ports))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// formatProtocolPorts formats a single protocol entry, e.g. "tcp:80,443" or just "icmp"
func formatProtocolPorts(protocol string, ports []string) string {
	if len(ports) == 0 {
		return protocol
	}
	return protocol + ":" + strings.Join(ports, ",")
}

// convertAllowed converts API allowed entries to our FirewallRuleEntry slice
func convertAllowed(items []*compute.FirewallAllowed) []FirewallRuleEntry {
	entries := make([]FirewallRuleEntry, len(items))
	for i, a := range items {
		entries[i] = FirewallRuleEntry{
			Protocol: a.IPProtocol,
			Ports:    a.Ports,
		}
	}
	return entries
}

// convertDenied converts API denied entries to our FirewallRuleEntry slice
func convertDenied(items []*compute.FirewallDenied) []FirewallRuleEntry {
	entries := make([]FirewallRuleEntry, len(items))
	for i, d := range items {
		entries[i] = FirewallRuleEntry{
			Protocol: d.IPProtocol,
			Ports:    d.Ports,
		}
	}
	return entries
}

// deriveAction determines whether a firewall rule is ALLOW or DENY
func deriveAction(fw *compute.Firewall) string {
	if len(fw.Allowed) > 0 {
		return "ALLOW"
	}
	if len(fw.Denied) > 0 {
		return "DENY"
	}
	return "UNKNOWN"
}
