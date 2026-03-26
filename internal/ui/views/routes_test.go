package views

import (
	"strings"
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouteToRow(t *testing.T) {
	tests := []struct {
		name     string
		route    gcp.Route
		validate func(t *testing.T, row table.Row)
	}{
		{
			name: "static route with IP next hop",
			route: gcp.Route{
				Name:      "custom-route-1",
				Network:   "default",
				DestRange: "10.0.0.0/8",
				Priority:  1000,
				NextHop:   "10.128.0.1",
				RouteType: "Static",
				CreatedAt: "2024-01-15T10:30:00Z",
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "custom-route-1", row.Data[0])
				assert.Equal(t, "default", row.Data[1])
				assert.Equal(t, "10.0.0.0/8", row.Data[2])
				assert.Equal(t, "1000", row.Data[3])
				assert.Equal(t, "10.128.0.1", row.Data[4])
				// Static type should NOT have muted styling applied as raw data
				assert.Equal(t, "Static", row.Data[5])
				assert.Equal(t, "custom-route-1", row.ID)
			},
		},
		{
			name: "system route with default gateway",
			route: gcp.Route{
				Name:      "default-route-abc123",
				Network:   "default",
				DestRange: "0.0.0.0/0",
				Priority:  1000,
				NextHop:   "Default internet gateway",
				RouteType: "System",
				CreatedAt: "2024-01-01T00:00:00Z",
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "default-route-abc123", row.Data[0])
				assert.Equal(t, "0.0.0.0/0", row.Data[2])
				assert.Equal(t, "Default internet gateway", row.Data[4])
				// System type should have muted style — contains ANSI escape codes
				assert.Contains(t, row.Data[5], "System")
				assert.Equal(t, "default-route-abc123", row.ID)
			},
		},
		{
			name: "subnet route",
			route: gcp.Route{
				Name:      "default-route-subnet",
				Network:   "prod-vpc",
				DestRange: "10.128.0.0/20",
				Priority:  0,
				NextHop:   "prod-vpc",
				RouteType: "Subnet",
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Equal(t, "prod-vpc", row.Data[1])
				assert.Equal(t, "0", row.Data[3], "Zero priority should render as 0")
				// Subnet type should have muted style
				assert.Contains(t, row.Data[5], "Subnet")
				assert.Equal(t, "default-route-subnet", row.ID)
			},
		},
		{
			name: "peering route",
			route: gcp.Route{
				Name:      "peering-route-1",
				Network:   "hub-vpc",
				DestRange: "172.16.0.0/16",
				Priority:  100,
				NextHop:   "spoke-vpc",
				RouteType: "Peering",
			},
			validate: func(t *testing.T, row table.Row) {
				// Peering type should have muted style
				assert.Contains(t, row.Data[5], "Peering")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := routeToRow(tt.route)
			tt.validate(t, row)
		})
	}
}

func TestRouteToRow_FilterValue(t *testing.T) {
	r := gcp.Route{
		Name:      "my-route",
		Network:   "my-network",
		DestRange: "10.0.0.0/24",
		RouteType: "Static",
	}

	row := routeToRow(r)

	// Filter value should contain all searchable fields
	assert.Contains(t, row.FilterValue, "my-route")
	assert.Contains(t, row.FilterValue, "my-network")
	assert.Contains(t, row.FilterValue, "10.0.0.0/24")
	assert.Contains(t, row.FilterValue, "Static")
}

func TestNewRoutesView(t *testing.T) {
	v := NewRoutesView("test-project")

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.True(t, v.loading, "View should start in loading state")
	assert.Nil(t, v.computeClient, "Client should be nil until async init completes")
	assert.False(t, v.menuOpen, "Menu should start closed")
	assert.False(t, v.showDeleteConfirm, "Delete confirm should start hidden")
}

func TestRoutesView_FindRouteByName(t *testing.T) {
	v := NewRoutesView("test-project")
	v.routes = []gcp.Route{
		{Name: "route-a", Network: "default", DestRange: "10.0.0.0/8"},
		{Name: "route-b", Network: "prod-vpc", DestRange: "172.16.0.0/16"},
	}

	// Found
	r, ok := v.findRouteByName("route-a")
	assert.True(t, ok)
	assert.Equal(t, "route-a", r.Name)
	assert.Equal(t, "default", r.Network)

	// Found second route
	r, ok = v.findRouteByName("route-b")
	assert.True(t, ok)
	assert.Equal(t, "prod-vpc", r.Network)

	// Not found
	_, ok = v.findRouteByName("nonexistent")
	assert.False(t, ok)
}

func TestRoutesView_HasTextInputFocused(t *testing.T) {
	v := NewRoutesView("test-project")

	// No filter or dialog active by default
	assert.False(t, v.HasTextInputFocused())
}

func TestRoutesView_IsMenuOpen(t *testing.T) {
	v := NewRoutesView("test-project")

	assert.False(t, v.IsMenuOpen(), "Menu should start closed")
}

func TestRoutesView_HelpText(t *testing.T) {
	v := NewRoutesView("test-project")
	v.loading = false
	v.routes = []gcp.Route{{Name: "route-1", Network: "default"}}

	rows := []table.Row{routeToRow(v.routes[0])}
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

func TestRoutesView_EmptyState(t *testing.T) {
	v := NewRoutesView("test-project")
	v.loading = false
	v.routes = []gcp.Route{}

	output := v.View()
	assert.Contains(t, output, "No routes found")
	assert.Contains(t, output, "Press 'c' to create a route")
}

func TestRoutesView_InitiateDelete_OnlyStaticAllowed(t *testing.T) {
	v := NewRoutesView("test-project")
	v.routes = []gcp.Route{
		{Name: "system-route", RouteType: "System", Network: "default"},
		{Name: "static-route", RouteType: "Static", Network: "default", DestRange: "10.0.0.0/8", Priority: 1000},
	}

	// Set up table rows so SelectedRow() works
	rows := make([]table.Row, len(v.routes))
	for i, r := range v.routes {
		rows[i] = routeToRow(r)
	}
	v.table.SetRows(rows)

	ctx := context.New()
	ctx.ContentWidth = 120
	ctx.ContentHeight = 40
	v.SetContext(ctx)

	// Select system route (first row, index 0) — delete should be a no-op
	cmd := v.initiateDelete()
	// System route should not open confirm dialog
	if v.routes[0].RouteType != "Static" {
		assert.Nil(t, cmd, "Should not allow deleting non-static routes")
		assert.False(t, v.showDeleteConfirm)
	}
}

func TestRoutesView_BuildActions_DeleteDisabledForNonStatic(t *testing.T) {
	v := NewRoutesView("test-project")
	v.routes = []gcp.Route{
		{Name: "system-route", RouteType: "System", Network: "default"},
	}

	rows := []table.Row{routeToRow(v.routes[0])}
	v.table.SetRows(rows)

	ctx := context.New()
	ctx.ContentWidth = 120
	ctx.ContentHeight = 40
	v.SetContext(ctx)

	actions := v.buildActions()

	// Find the delete action
	var deleteAction actionmenu.Action
	for _, a := range actions {
		if a.Key == 'D' {
			deleteAction = a
			break
		}
	}

	assert.False(t, deleteAction.Enabled, "Delete should be disabled for system routes")
}

// --- Route Details View Tests ---

func TestNewRouteDetailsView(t *testing.T) {
	v := NewRouteDetailsView("test-project", "my-route", nil)

	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.Equal(t, "my-route", v.routeName)
	assert.True(t, v.loading, "View should start in loading state")
	assert.Nil(t, v.details, "Details should start nil")
	assert.NotNil(t, v.networkLink, "Network link should be initialized")
	assert.False(t, v.menuOpen, "Menu should start closed")
	assert.Nil(t, v.GetComputeClient(), "Compute client should be nil when not set")
	assert.Equal(t, "my-route", v.GetRouteName())
	assert.False(t, v.IsMenuOpen())
	assert.False(t, v.HasTextInputFocused())
}

func TestRouteDetailsRenderContent(t *testing.T) {
	v := NewRouteDetailsView("test-project", "my-route", nil)
	v.details = &gcp.RouteDetails{
		Name:        "my-route",
		Description: "Test route description",
		Network:     "default",
		DestRange:   "10.0.0.0/8",
		Priority:    1000,
		NextHop:     "Default internet gateway",
		NextHopType: "Gateway",
		RouteType:   "Static",
		Tags:        []string{"web-server", "api"},
		CreatedAt:   "2025-01-15T10:30:00.000-07:00",
		ID:          123456789,
		SelfLink:    "https://compute.googleapis.com/compute/v1/projects/test/global/routes/my-route",
	}

	// Set up viewport so renderContent works
	v.width = 80
	v.height = 40
	v.applySize(v.width, v.height)

	content := v.renderContent()
	require.NotEmpty(t, content)

	// Header
	assert.Contains(t, content, "Route: my-route")

	// Basic Information section
	assert.Contains(t, content, "Basic Information")
	assert.Contains(t, content, "123456789")
	assert.Contains(t, content, "Test route description")

	// Routing section
	assert.Contains(t, content, "Routing")
	assert.Contains(t, content, "10.0.0.0/8")
	assert.Contains(t, content, "1000")
	assert.Contains(t, content, "Gateway")
	assert.Contains(t, content, "Default internet gateway")
	assert.Contains(t, content, "Static")
	assert.Contains(t, content, "web-server, api")

	// Network section
	assert.Contains(t, content, "Network")
	assert.Contains(t, content, "default")
}

func TestRouteDetailsRenderContent_NoTags(t *testing.T) {
	v := NewRouteDetailsView("test-project", "tagless-route", nil)
	v.details = &gcp.RouteDetails{
		Name:      "tagless-route",
		Network:   "default",
		DestRange: "0.0.0.0/0",
		Priority:  1000,
		NextHop:   "Default internet gateway",
		RouteType: "System",
	}

	v.width = 80
	v.height = 40
	v.applySize(v.width, v.height)

	content := v.renderContent()
	require.NotEmpty(t, content)

	assert.True(t, strings.Contains(content, "None"), "Should show 'None' for empty tags")
}

func TestRouteDetailsRenderContent_WithWarnings(t *testing.T) {
	v := NewRouteDetailsView("test-project", "warn-route", nil)
	v.details = &gcp.RouteDetails{
		Name:      "warn-route",
		Network:   "default",
		DestRange: "10.0.0.0/8",
		Priority:  100,
		NextHop:   "10.0.0.1",
		RouteType: "Static",
		Warnings:  []string{"Next hop not running", "Route may be unreachable"},
	}

	v.width = 80
	v.height = 40
	v.applySize(v.width, v.height)

	content := v.renderContent()
	require.NotEmpty(t, content)

	assert.Contains(t, content, "Warnings")
	assert.Contains(t, content, "Next hop not running")
	assert.Contains(t, content, "Route may be unreachable")
}

func TestRouteDetailsRenderContent_NoWarnings(t *testing.T) {
	v := NewRouteDetailsView("test-project", "no-warn-route", nil)
	v.details = &gcp.RouteDetails{
		Name:      "no-warn-route",
		Network:   "default",
		DestRange: "10.0.0.0/8",
		Priority:  100,
		NextHop:   "Default internet gateway",
		RouteType: "System",
		Warnings:  nil,
	}

	v.width = 80
	v.height = 40
	v.applySize(v.width, v.height)

	content := v.renderContent()
	require.NotEmpty(t, content)

	// "Warnings" section should NOT appear when no warnings
	assert.NotContains(t, content, "Warnings")
}

func TestRouteDetailsView_DeleteOnlyForStatic(t *testing.T) {
	// Static route — delete should be allowed
	v := NewRouteDetailsView("test-project", "static-route", nil)
	v.details = &gcp.RouteDetails{
		Name:      "static-route",
		Network:   "default",
		DestRange: "10.0.0.0/8",
		Priority:  100,
		RouteType: "Static",
	}

	cmd := v.initiateDelete()
	assert.NotNil(t, cmd, "Delete should be allowed for static routes")
	assert.True(t, v.showDeleteConfirm, "Should show delete confirmation for static routes")

	// System route — delete should be blocked
	v2 := NewRouteDetailsView("test-project", "system-route", nil)
	v2.details = &gcp.RouteDetails{
		Name:      "system-route",
		Network:   "default",
		DestRange: "0.0.0.0/0",
		Priority:  1000,
		RouteType: "System",
	}

	cmd2 := v2.initiateDelete()
	assert.Nil(t, cmd2, "Delete should be blocked for system routes")
	assert.False(t, v2.showDeleteConfirm, "Should not show delete confirmation for system routes")
}

func TestRouteDetailsView_BuildActions(t *testing.T) {
	// Static route — delete should be enabled
	v := NewRouteDetailsView("test-project", "static-route", nil)
	v.details = &gcp.RouteDetails{
		Name:      "static-route",
		RouteType: "Static",
	}

	actions := v.buildActions()
	assert.Len(t, actions, 2)
	assert.Equal(t, 'r', actions[0].Key)
	assert.True(t, actions[0].Enabled)
	assert.Equal(t, 'D', actions[1].Key)
	assert.True(t, actions[1].Enabled, "Delete should be enabled for static routes")

	// Subnet route — delete should be disabled
	v2 := NewRouteDetailsView("test-project", "subnet-route", nil)
	v2.details = &gcp.RouteDetails{
		Name:      "subnet-route",
		RouteType: "Subnet",
	}

	actions2 := v2.buildActions()
	assert.False(t, actions2[1].Enabled, "Delete should be disabled for non-static routes")
}
