package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
)

func TestSubnetToRow(t *testing.T) {
	tests := []struct {
		name     string
		subnet   gcp.Subnet
		validate func(t *testing.T, row table.Row)
	}{
		{
			name: "standard private subnet",
			subnet: gcp.Subnet{
				Name:                  "default",
				Network:               "default",
				Region:                "us-central1",
				IPCidrRange:           "10.128.0.0/20",
				Purpose:               "PRIVATE",
				PrivateIPGoogleAccess: true,
				EnableFlowLogs:        false,
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "default", row.Data[0])
				assert.Equal(t, "default", row.Data[1])
				assert.Equal(t, "us-central1", row.Data[2])
				assert.Equal(t, "10.128.0.0/20", row.Data[3])
				assert.Equal(t, "Private", row.Data[4])
				assert.Equal(t, "✓", row.Data[5], "PrivateIPGoogleAccess should show check")
				assert.Equal(t, "—", row.Data[6], "FlowLogs disabled should show dash")
				assert.Equal(t, "default/us-central1", row.ID)
			},
		},
		{
			name: "managed proxy subnet with flow logs",
			subnet: gcp.Subnet{
				Name:                  "proxy-subnet",
				Network:               "prod-vpc",
				Region:                "europe-west1",
				IPCidrRange:           "10.0.0.0/24",
				Purpose:               "REGIONAL_MANAGED_PROXY",
				PrivateIPGoogleAccess: false,
				EnableFlowLogs:        true,
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "proxy-subnet", row.Data[0])
				assert.Equal(t, "prod-vpc", row.Data[1])
				assert.Equal(t, "europe-west1", row.Data[2])
				assert.Equal(t, "10.0.0.0/24", row.Data[3])
				assert.Equal(t, "Managed Proxy", row.Data[4])
				assert.Equal(t, "—", row.Data[5], "PrivateIPGoogleAccess disabled should show dash")
				assert.Equal(t, "✓", row.Data[6], "FlowLogs enabled should show check")
				assert.Equal(t, "proxy-subnet/europe-west1", row.ID)
			},
		},
		{
			name: "subnet with empty purpose",
			subnet: gcp.Subnet{
				Name:        "legacy-subnet",
				Network:     "legacy-vpc",
				Region:      "asia-east1",
				IPCidrRange: "172.16.0.0/16",
				Purpose:     "",
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "—", row.Data[4], "Empty purpose should show dash")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := subnetToRow(tt.subnet)
			tt.validate(t, row)
		})
	}
}

func TestSubnetToRow_FilterValue(t *testing.T) {
	s := gcp.Subnet{
		Name:        "my-subnet",
		Network:     "my-network",
		Region:      "us-west1",
		IPCidrRange: "10.0.0.0/24",
		Purpose:     "PRIVATE",
	}

	row := subnetToRow(s)

	// Filter value should contain all searchable fields
	assert.Contains(t, row.FilterValue, "my-subnet")
	assert.Contains(t, row.FilterValue, "my-network")
	assert.Contains(t, row.FilterValue, "us-west1")
	assert.Contains(t, row.FilterValue, "10.0.0.0/24")
	assert.Contains(t, row.FilterValue, "PRIVATE")
}

func TestSubnetsView_FormatSubnetPurpose(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"PRIVATE", "Private"},
		{"REGIONAL_MANAGED_PROXY", "Managed Proxy"},
		{"GLOBAL_MANAGED_PROXY", "Global Proxy"},
		{"INTERNAL_HTTPS_LOAD_BALANCER", "Internal LB"},
		{"PRIVATE_SERVICE_CONNECT", "PSC"},
		{"", "—"},
		{"UNKNOWN_PURPOSE", "UNKNOWN_PURPOSE"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatSubnetPurpose(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBoolCheck(t *testing.T) {
	assert.Equal(t, "✓", formatBoolCheck(true))
	assert.Equal(t, "—", formatBoolCheck(false))
}

func TestNewSubnetsView(t *testing.T) {
	v := NewSubnetsView("test-project")

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.True(t, v.loading, "View should start in loading state")
	assert.Nil(t, v.computeClient, "Client should be nil until async init completes")
	assert.False(t, v.menuOpen, "Menu should start closed")
	assert.False(t, v.showDeleteConfirm, "Delete confirm should start hidden")
}

func TestSubnetsView_FindSubnetByID(t *testing.T) {
	v := NewSubnetsView("test-project")
	v.subnets = []gcp.Subnet{
		{Name: "default", Region: "us-central1", Network: "default"},
		{Name: "default", Region: "europe-west1", Network: "default"},
		{Name: "custom", Region: "us-central1", Network: "prod-vpc"},
	}

	// Found by composite ID
	s, ok := v.findSubnetByID("default/us-central1")
	assert.True(t, ok)
	assert.Equal(t, "default", s.Name)
	assert.Equal(t, "us-central1", s.Region)

	// Same name, different region
	s, ok = v.findSubnetByID("default/europe-west1")
	assert.True(t, ok)
	assert.Equal(t, "europe-west1", s.Region)

	// Not found
	_, ok = v.findSubnetByID("nonexistent/us-central1")
	assert.False(t, ok)
}

func TestSubnetsView_HasTextInputFocused(t *testing.T) {
	v := NewSubnetsView("test-project")

	// No filter or dialog active by default
	assert.False(t, v.HasTextInputFocused())
}

func TestSubnetsView_IsMenuOpen(t *testing.T) {
	v := NewSubnetsView("test-project")

	assert.False(t, v.IsMenuOpen(), "Menu should start closed")
}

func TestSubnetsView_HelpText(t *testing.T) {
	v := NewSubnetsView("test-project")
	v.loading = false
	v.subnets = []gcp.Subnet{{Name: "default", Region: "us-central1"}}

	rows := []table.Row{subnetToRow(v.subnets[0])}
	v.table.SetRows(rows)

	ctx := context.New()
	ctx.ContentWidth = 120
	ctx.ContentHeight = 40
	v.SetContext(ctx)

	output := v.View()
	assert.Contains(t, output, "enter: details")
	assert.Contains(t, output, "c: create")
	assert.Contains(t, output, "D: delete")
}

func TestSubnetsView_EmptyState(t *testing.T) {
	v := NewSubnetsView("test-project")
	v.loading = false
	v.subnets = []gcp.Subnet{}

	output := v.View()
	assert.Contains(t, output, "No subnets found")
	assert.Contains(t, output, "Press 'c' to create a subnet")
}
