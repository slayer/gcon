package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/compute/v1"
)

func TestSummarizeProtocols(t *testing.T) {
	tests := []struct {
		name     string
		allowed  []*compute.FirewallAllowed
		denied   []*compute.FirewallDenied
		expected string
	}{
		{
			name: "single protocol no ports",
			allowed: []*compute.FirewallAllowed{
				{IPProtocol: "tcp"},
			},
			denied:   nil,
			expected: "tcp",
		},
		{
			name: "protocol with ports",
			allowed: []*compute.FirewallAllowed{
				{IPProtocol: "tcp", Ports: []string{"80", "443"}},
			},
			denied:   nil,
			expected: "tcp:80,443",
		},
		{
			name: "multiple protocols",
			allowed: []*compute.FirewallAllowed{
				{IPProtocol: "tcp", Ports: []string{"80"}},
				{IPProtocol: "icmp"},
			},
			denied:   nil,
			expected: "tcp:80 icmp",
		},
		{
			name: "all protocol",
			allowed: []*compute.FirewallAllowed{
				{IPProtocol: "all"},
			},
			denied:   nil,
			expected: "all",
		},
		{
			name:     "empty lists",
			allowed:  nil,
			denied:   nil,
			expected: "",
		},
		{
			name:    "only denied entries",
			allowed: nil,
			denied: []*compute.FirewallDenied{
				{IPProtocol: "tcp", Ports: []string{"22"}},
				{IPProtocol: "udp", Ports: []string{"53"}},
			},
			expected: "tcp:22 udp:53",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := summarizeProtocols(tt.allowed, tt.denied)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeriveAction(t *testing.T) {
	tests := []struct {
		name     string
		fw       *compute.Firewall
		expected string
	}{
		{
			name: "allowed non-empty",
			fw: &compute.Firewall{
				Allowed: []*compute.FirewallAllowed{
					{IPProtocol: "tcp"},
				},
			},
			expected: "ALLOW",
		},
		{
			name: "denied non-empty",
			fw: &compute.Firewall{
				Denied: []*compute.FirewallDenied{
					{IPProtocol: "tcp"},
				},
			},
			expected: "DENY",
		},
		{
			name:     "both empty",
			fw:       &compute.Firewall{},
			expected: "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveAction(tt.fw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFirewallRuleFromAPI(t *testing.T) {
	fw := &compute.Firewall{
		Name:      "allow-http",
		Id:        99001,
		Direction: "INGRESS",
		Priority:  1000,
		Disabled:  true,
		Network:   "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/my-vpc",
		Allowed: []*compute.FirewallAllowed{
			{IPProtocol: "tcp", Ports: []string{"80", "443"}},
		},
		CreationTimestamp: "2024-03-10T08:00:00.000-07:00",
	}

	rule := firewallRuleFromAPI(fw)

	assert.Equal(t, "allow-http", rule.Name)
	assert.Equal(t, uint64(99001), rule.ID)
	assert.Equal(t, "INGRESS", rule.Direction)
	assert.Equal(t, "ALLOW", rule.Action)
	assert.Equal(t, int64(1000), rule.Priority)
	assert.Equal(t, "tcp:80,443", rule.Protocols)
	assert.Equal(t, "my-vpc", rule.Network)
	assert.True(t, rule.Disabled)
	assert.Equal(t, "2024-03-10T08:00:00.000-07:00", rule.CreatedAt)
}

func TestFirewallRuleDetailsFromAPI(t *testing.T) {
	fw := &compute.Firewall{
		Name:        "deny-ssh",
		Id:          88002,
		Description: "Block SSH from internet",
		Direction:   "INGRESS",
		Priority:    500,
		Disabled:    false,
		Network:     "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/prod-vpc",
		Denied: []*compute.FirewallDenied{
			{IPProtocol: "tcp", Ports: []string{"22"}},
		},
		SourceRanges:          []string{"0.0.0.0/0"},
		DestinationRanges:     []string{"10.0.0.0/8"},
		SourceTags:            []string{"external"},
		TargetTags:            []string{"no-ssh"},
		SourceServiceAccounts: []string{"sa-1@project.iam.gserviceaccount.com"},
		TargetServiceAccounts: []string{"sa-2@project.iam.gserviceaccount.com"},
		LogConfig: &compute.FirewallLogConfig{
			Enable:   true,
			Metadata: "INCLUDE_ALL_METADATA",
		},
		CreationTimestamp: "2024-05-20T14:30:00.000-07:00",
		SelfLink:          "https://www.googleapis.com/compute/v1/projects/my-project/global/firewalls/deny-ssh",
	}

	details := firewallRuleDetailsFromAPI(fw)

	assert.Equal(t, "deny-ssh", details.Name)
	assert.Equal(t, uint64(88002), details.ID)
	assert.Equal(t, "Block SSH from internet", details.Description)
	assert.Equal(t, "INGRESS", details.Direction)
	assert.Equal(t, "DENY", details.Action)
	assert.Equal(t, int64(500), details.Priority)
	assert.False(t, details.Disabled)
	assert.Equal(t, "prod-vpc", details.Network)
	assert.Equal(t, "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/prod-vpc", details.NetworkURL)

	// Allowed should be empty, Denied should have the entry
	assert.Empty(t, details.Allowed)
	assert.Len(t, details.Denied, 1)
	assert.Equal(t, "tcp", details.Denied[0].Protocol)
	assert.Equal(t, []string{"22"}, details.Denied[0].Ports)

	// Range and tag fields
	assert.Equal(t, []string{"0.0.0.0/0"}, details.SourceRanges)
	assert.Equal(t, []string{"10.0.0.0/8"}, details.DestinationRanges)
	assert.Equal(t, []string{"external"}, details.SourceTags)
	assert.Equal(t, []string{"no-ssh"}, details.TargetTags)
	assert.Equal(t, []string{"sa-1@project.iam.gserviceaccount.com"}, details.SourceServiceAccounts)
	assert.Equal(t, []string{"sa-2@project.iam.gserviceaccount.com"}, details.TargetServiceAccounts)

	// LogConfig
	assert.True(t, details.LogEnabled)
	assert.Equal(t, "INCLUDE_ALL_METADATA", details.LogMetadata)

	assert.Equal(t, "2024-05-20T14:30:00.000-07:00", details.CreatedAt)
	assert.Equal(t, "https://www.googleapis.com/compute/v1/projects/my-project/global/firewalls/deny-ssh", details.SelfLink)
}

func TestFirewallRuleDetailsFromAPI_NilLogConfig(t *testing.T) {
	fw := &compute.Firewall{
		Name:      "no-logs-rule",
		LogConfig: nil,
		Allowed: []*compute.FirewallAllowed{
			{IPProtocol: "icmp"},
		},
	}

	details := firewallRuleDetailsFromAPI(fw)

	assert.False(t, details.LogEnabled)
	assert.Equal(t, "", details.LogMetadata)
}

func TestFirewallRuleDetailsFromAPI_NilSlices(t *testing.T) {
	// Verify nil API slices produce empty (not nil) entry slices
	fw := &compute.Firewall{
		Name: "empty-rule",
	}

	details := firewallRuleDetailsFromAPI(fw)

	assert.NotNil(t, details.Allowed)
	assert.NotNil(t, details.Denied)
	assert.Empty(t, details.Allowed)
	assert.Empty(t, details.Denied)
}

func TestFormatProtocolPorts(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		ports    []string
		expected string
	}{
		{
			name:     "no ports",
			protocol: "tcp",
			ports:    nil,
			expected: "tcp",
		},
		{
			name:     "with ports",
			protocol: "tcp",
			ports:    []string{"80", "443"},
			expected: "tcp:80,443",
		},
		{
			name:     "single port",
			protocol: "udp",
			ports:    []string{"53"},
			expected: "udp:53",
		},
		{
			name:     "empty ports slice",
			protocol: "icmp",
			ports:    []string{},
			expected: "icmp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatProtocolPorts(tt.protocol, tt.ports)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertAllowed(t *testing.T) {
	items := []*compute.FirewallAllowed{
		{IPProtocol: "tcp", Ports: []string{"80", "443"}},
		{IPProtocol: "icmp"},
	}

	entries := convertAllowed(items)

	assert.Len(t, entries, 2)
	assert.Equal(t, "tcp", entries[0].Protocol)
	assert.Equal(t, []string{"80", "443"}, entries[0].Ports)
	assert.Equal(t, "icmp", entries[1].Protocol)
	assert.Nil(t, entries[1].Ports)
}

func TestConvertDenied(t *testing.T) {
	items := []*compute.FirewallDenied{
		{IPProtocol: "udp", Ports: []string{"53"}},
		{IPProtocol: "tcp", Ports: []string{"22", "3389"}},
	}

	entries := convertDenied(items)

	assert.Len(t, entries, 2)
	assert.Equal(t, "udp", entries[0].Protocol)
	assert.Equal(t, []string{"53"}, entries[0].Ports)
	assert.Equal(t, "tcp", entries[1].Protocol)
	assert.Equal(t, []string{"22", "3389"}, entries[1].Ports)
}
