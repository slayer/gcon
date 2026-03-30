package views

import "github.com/slayer/gcon/internal/gcp"

// RoutesRequestMsg navigates to the routes list view
type RoutesRequestMsg struct{}

// RouteSelectedMsg navigates to route details
type RouteSelectedMsg struct {
	Route gcp.Route
}

// RouteCreateRequestMsg opens the route creation form.
// Network is optional — when set, pre-fills the network dropdown.
type RouteCreateRequestMsg struct {
	Network string
}

// RouteCreateResultMsg reports the result of an async route creation
type RouteCreateResultMsg struct {
	Error error
	Name  string
}

// RouteCreateCanceledMsg is sent when user cancels route creation
type RouteCreateCanceledMsg struct{}

// RouteDeleteRequestMsg triggers route deletion with confirmation
type RouteDeleteRequestMsg struct {
	Name string
}

// RouteDeleteResultMsg reports the result of an async route deletion
type RouteDeleteResultMsg struct {
	Error error
	Name  string
}

// CreateRouteMsg is emitted by the create form for the app handler to execute
type CreateRouteMsg struct {
	Config gcp.RouteConfig
}
