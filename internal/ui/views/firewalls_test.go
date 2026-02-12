package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
)

func TestFirewallsView_NewFirewallsView(t *testing.T) {
	v := NewFirewallsView("test-project")

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.True(t, v.loading, "View should start in loading state")
}

func TestFirewallsView_RenderLoading(t *testing.T) {
	v := NewFirewallsView("test-project")
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30}
	v.SetContext(ctx)

	output := renderLoading(v.spinner, "Loading firewall rules...")

	assert.Contains(t, output, "Loading firewall rules...")
	assert.Contains(t, output, v.spinner.View())
}

func TestFirewallToRow(t *testing.T) {
	tests := []struct {
		name     string
		firewall gcp.FirewallRule
		validate func(t *testing.T, row table.Row)
	}{
		{
			name: "enabled ingress allow rule",
			firewall: gcp.FirewallRule{
				Name:      "allow-ssh",
				Direction: "INGRESS",
				Priority:  1000,
				Action:    "ALLOW",
				Protocols: "tcp:22",
				Network:   "default",
				Disabled:  false,
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "allow-ssh", row.Data[0])
				assert.Equal(t, "INGRESS", row.Data[1])
				assert.Equal(t, "1000", row.Data[2])
				assert.Equal(t, "ALLOW", row.Data[3])
				assert.Equal(t, "tcp:22", row.Data[4])
				assert.Equal(t, "default", row.Data[5])
				// Enabled rule should have a non-empty status indicator
				assert.NotEmpty(t, row.Data[6], "Status column should contain a status string")
				assert.Equal(t, "allow-ssh", row.ID)
			},
		},
		{
			name: "disabled egress deny rule",
			firewall: gcp.FirewallRule{
				Name:      "deny-all-egress",
				Direction: "EGRESS",
				Priority:  65534,
				Action:    "DENY",
				Protocols: "all",
				Network:   "custom-vpc",
				Disabled:  true,
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "deny-all-egress", row.Data[0])
				assert.Equal(t, "EGRESS", row.Data[1])
				assert.Equal(t, "65534", row.Data[2])
				assert.Equal(t, "DENY", row.Data[3])
				assert.Equal(t, "all", row.Data[4])
				assert.Equal(t, "custom-vpc", row.Data[5])
				// Disabled rule should also have a non-empty status indicator
				assert.NotEmpty(t, row.Data[6], "Status column should contain a status string")
				assert.Equal(t, "deny-all-egress", row.ID)
			},
		},
		{
			name: "rule with multiple protocols",
			firewall: gcp.FirewallRule{
				Name:      "allow-web",
				Direction: "INGRESS",
				Priority:  500,
				Action:    "ALLOW",
				Protocols: "tcp:80,443 icmp",
				Network:   "prod-vpc",
				Disabled:  false,
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "allow-web", row.Data[0])
				assert.Equal(t, "tcp:80,443 icmp", row.Data[4])
				assert.Equal(t, "prod-vpc", row.Data[5])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := firewallToRow(tt.firewall)
			tt.validate(t, row)
		})
	}
}

func TestFirewallToRow_FilterValue(t *testing.T) {
	fw := gcp.FirewallRule{
		Name:      "allow-http",
		Direction: "INGRESS",
		Action:    "ALLOW",
		Network:   "default",
		Protocols: "tcp:80,443",
	}

	row := firewallToRow(fw)

	// Filter value should contain all searchable fields
	assert.Contains(t, row.FilterValue, "allow-http")
	assert.Contains(t, row.FilterValue, "INGRESS")
	assert.Contains(t, row.FilterValue, "ALLOW")
	assert.Contains(t, row.FilterValue, "default")
	assert.Contains(t, row.FilterValue, "tcp:80,443")
}

func TestFirewallsView_FindFirewallByName(t *testing.T) {
	v := NewFirewallsView("test-project")
	v.firewalls = []gcp.FirewallRule{
		{Name: "allow-ssh", Direction: "INGRESS"},
		{Name: "deny-all", Direction: "EGRESS"},
	}

	// Found
	fw, ok := v.findFirewallByName("deny-all")
	assert.True(t, ok)
	assert.Equal(t, "deny-all", fw.Name)

	// Not found
	_, ok = v.findFirewallByName("nonexistent")
	assert.False(t, ok)
}

func TestFirewallsView_HasTextInputFocused(t *testing.T) {
	v := NewFirewallsView("test-project")

	// No filter or dialog active by default
	assert.False(t, v.HasTextInputFocused())
}

func TestFirewallsView_IsMenuOpen(t *testing.T) {
	v := NewFirewallsView("test-project")

	assert.False(t, v.IsMenuOpen(), "Menu should start closed")
}

func TestFirewallsView_HelpText(t *testing.T) {
	v := NewFirewallsView("test-project")
	v.loading = false
	v.firewalls = []gcp.FirewallRule{{Name: "allow-ssh"}}

	rows := []table.Row{firewallToRow(v.firewalls[0])}
	v.table.SetRows(rows)

	ctx := &context.ProgramContext{ContentWidth: 100, ContentHeight: 40}
	v.SetContext(ctx)

	output := v.View()
	assert.Contains(t, output, "enter: details")
}
