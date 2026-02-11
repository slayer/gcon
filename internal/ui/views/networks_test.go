package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
)

func TestNetworksView_NewNetworksView(t *testing.T) {
	v := NewNetworksView("test-project")

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.True(t, v.loading, "View should start in loading state")
}

func TestNetworksView_RenderLoading(t *testing.T) {
	v := NewNetworksView("test-project")
	ctx := &context.ProgramContext{ContentWidth: 80, ContentHeight: 30}
	v.SetContext(ctx)

	output := renderLoading(v.spinner, "Loading VPC networks...")

	assert.Contains(t, output, "Loading VPC networks...")
	assert.Contains(t, output, v.spinner.View())
}

func TestNetworkToRow(t *testing.T) {
	tests := []struct {
		name     string
		network  gcp.Network
		validate func(t *testing.T, row table.Row)
	}{
		{
			name: "auto mode with regional routing",
			network: gcp.Network{
				Name:         "default",
				AutoCreate:   true,
				RoutingMode:  "REGIONAL",
				SubnetsCount: 28,
				CreatedAt:    "2023-01-15T10:00:00.000-07:00",
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "default", row.Data[0])
				assert.Equal(t, "Auto", row.Data[1])
				assert.Equal(t, "REGIONAL", row.Data[2])
				assert.Equal(t, "28", row.Data[3])
				assert.Equal(t, "2023-01-15T10:00:00.000-07:00", row.Data[4])
				assert.Equal(t, "default", row.ID)
			},
		},
		{
			name: "custom mode with global routing",
			network: gcp.Network{
				Name:         "custom-vpc",
				AutoCreate:   false,
				RoutingMode:  "GLOBAL",
				SubnetsCount: 5,
				CreatedAt:    "2024-06-20T15:30:00.000-07:00",
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "custom-vpc", row.Data[0])
				assert.Equal(t, "Custom", row.Data[1])
				assert.Equal(t, "GLOBAL", row.Data[2])
				assert.Equal(t, "5", row.Data[3])
				assert.Equal(t, "custom-vpc", row.ID)
			},
		},
		{
			name: "custom mode with no subnets",
			network: gcp.Network{
				Name:         "empty-vpc",
				AutoCreate:   false,
				RoutingMode:  "REGIONAL",
				SubnetsCount: 0,
				CreatedAt:    "2025-03-10T08:00:00.000-07:00",
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "empty-vpc", row.Data[0])
				assert.Equal(t, "Custom", row.Data[1])
				assert.Equal(t, "0", row.Data[3])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := networkToRow(tt.network)
			tt.validate(t, row)
		})
	}
}

func TestNetworkToRow_FilterValue(t *testing.T) {
	network := gcp.Network{
		Name:        "my-vpc",
		AutoCreate:  true,
		RoutingMode: "GLOBAL",
	}

	row := networkToRow(network)

	// Filter value should contain name, mode, and routing for searchability
	assert.Contains(t, row.FilterValue, "my-vpc")
	assert.Contains(t, row.FilterValue, "Auto")
	assert.Contains(t, row.FilterValue, "GLOBAL")
}

func TestNetworksView_FindNetworkByID(t *testing.T) {
	v := NewNetworksView("test-project")
	v.networks = []gcp.Network{
		{Name: "default", ID: 1},
		{Name: "custom-vpc", ID: 2},
	}

	// Found
	network, ok := v.findNetworkByID("custom-vpc")
	assert.True(t, ok)
	assert.Equal(t, "custom-vpc", network.Name)

	// Not found
	_, ok = v.findNetworkByID("nonexistent")
	assert.False(t, ok)
}

func TestNetworksView_HelpTextIncludesEnter(t *testing.T) {
	v := NewNetworksView("test-project")
	v.loading = false
	v.networks = []gcp.Network{{Name: "default"}}

	rows := []table.Row{networkToRow(v.networks[0])}
	v.table.SetRows(rows)

	ctx := &context.ProgramContext{ContentWidth: 100, ContentHeight: 40}
	v.SetContext(ctx)

	output := v.View()
	assert.Contains(t, output, "enter: details")
}
