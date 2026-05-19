package views

import (
	container "google.golang.org/api/container/v1"

	tea "github.com/charmbracelet/bubbletea"
)

// === Node pool create ===

// GKENodePoolCreateRequestMsg is emitted by the create form on submit.
// The app handler calls gcp.CreateNodePool and registers polling.
type GKENodePoolCreateRequestMsg struct {
	ProjectID   string
	Location    string
	ClusterName string
	Pool        *container.NodePool // wrapped to preserve every form field
}

type GKENodePoolCreateCanceledMsg struct{}
type GKENodePoolCreateResultMsg struct {
	TaskID string
	Pool   string
	Error  error
}

// === Node pool delete ===

type GKENodePoolDeleteRequestMsg struct {
	ProjectID, Location, ClusterName, PoolName string
}

type GKENodePoolDeleteResultMsg struct {
	TaskID string
	Pool   string
	Error  error
}

// === Node pool resize ===

type GKENodePoolResizeMode int

const (
	GKENodePoolResizeManual    GKENodePoolResizeMode = iota
	GKENodePoolResizeAutoscale
)

type GKENodePoolResizeRequestMsg struct {
	ProjectID, Location, ClusterName, PoolName string
	Mode                                       GKENodePoolResizeMode
	NodeCount                                  int64 // manual mode
	AutoscaleEnabled                           bool  // autoscale mode
	MinNodes, MaxNodes                         int64 // autoscale mode
}

type GKENodePoolResizeResultMsg struct {
	TaskID string
	Pool   string
	Error  error
}

// === Upgrade (master + node pool, share the same dialog) ===

type GKEMasterUpgradeRequestMsg struct {
	ProjectID, Location, ClusterName, Version string
}
type GKEMasterUpgradeResultMsg struct {
	TaskID string
	Error  error
}

type GKENodePoolUpgradeRequestMsg struct {
	ProjectID, Location, ClusterName, PoolName, Version string
}
type GKENodePoolUpgradeResultMsg struct {
	TaskID string
	Pool   string
	Error  error
}

// === Node pool create (navigation from cluster details) ===

// GKENodePoolCreateRequestedMsg asks the app to open the create-pool form view.
// Emitted by GKEClusterDetailsView when the user presses `c` on the Node Pools tab.
type GKENodePoolCreateRequestedMsg struct {
	ProjectID, Location, ClusterName string
}

// === Operation polling ===

// GKEOperationPollMsg fires after a 5 s tick to re-fetch operation state.
// OnDone is the cmd to run when Status=="DONE" (e.g. a Refresh for the
// affected view). Stored as a function so the same poller can target
// different views.
type GKEOperationPollMsg struct {
	ProjectID, Location, Name string
	TaskID                    string
	OnDone                    func() tea.Cmd
	OnError                   func(error) tea.Cmd
}
