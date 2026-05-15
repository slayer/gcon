// internal/ui/views/gke_messages.go
package views

import "github.com/slayer/gcon/internal/gcp"

// GKEClusterSelectedMsg is emitted when a row is activated on the
// clusters list. The app handler navigates to the details view.
type GKEClusterSelectedMsg struct {
	ProjectID string
	Location  string
	Name      string
}

// GKEClusterDeleteRequestMsg is emitted after the user confirms the
// delete dialog. The app handler runs the API call.
type GKEClusterDeleteRequestMsg struct {
	ProjectID string
	Location  string
	Name      string
}

// GKEClusterActionResultMsg carries the outcome of a cluster action
// back to the view. Action is currently only "delete".
type GKEClusterActionResultMsg struct {
	Action string
	Error  error
}

// gkeClustersClientReadyMsg / gkeClustersErrorMsg / gkeClustersLoadedMsg
// are the internal load lifecycle for the list view.
type gkeClustersClientReadyMsg struct{ client *gcp.ContainerClient }
type gkeClustersErrorMsg struct{ err error }
type gkeClustersLoadedMsg struct{ clusters []gcp.Cluster }

// gkeClusterLoadedMsg / gkeClusterErrorMsg / gkeClusterClientReadyMsg are
// the internal load lifecycle for the details view.
type gkeClusterClientReadyMsg struct{ client *gcp.ContainerClient }
type gkeClusterLoadedMsg struct{ details *gcp.ClusterDetails }
type gkeClusterErrorMsg struct{ err error }
