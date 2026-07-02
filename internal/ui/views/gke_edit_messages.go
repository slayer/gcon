package views

import "github.com/slayer/gcon/internal/gcp"

// === Cluster edit ===

// GKEClusterEditOpenMsg is emitted by the cluster details view (`e` on
// Overview tab) to ask the app to open the cluster edit form.
type GKEClusterEditOpenMsg struct {
	ProjectID, Location, ClusterName string
}

// GKEClusterEditRequestMsg is emitted by the cluster edit form on confirm-deploy.
// Basic and Maintenance are independent — either, neither (defensive), or both
// can be set. When both are set, the app handler dispatches them as sequential
// steps under one footer task (see runGKEEditSequence).
type GKEClusterEditRequestMsg struct {
	ProjectID, Location, ClusterName string
	Basic                            *gcp.ClusterEdit
	Maintenance                      *gcp.MaintenanceWindow
}

// GKEClusterEditCanceledMsg is emitted on Esc from the form / diff view.
type GKEClusterEditCanceledMsg struct{}

// GKEClusterEditResultMsg carries the outcome of the (possibly multi-step)
// cluster edit. TaskID lets the app's result handler call finishTask on the
// footer task that was registered for this edit.
type GKEClusterEditResultMsg struct {
	TaskID, ClusterName string
	Error               error
}

// === Node pool edit ===

// GKENodePoolEditOpenMsg is emitted by the cluster details view (`e` on
// Node Pools tab, Standard clusters only) to ask the app to open the pool
// edit form for the focused pool.
type GKENodePoolEditOpenMsg struct {
	ProjectID, Location, ClusterName, PoolName string
}

// GKENodePoolEditRequestMsg is emitted by the pool edit form on confirm-deploy.
// Fields (labels / taints / upgrade settings) and Management (autoUpgrade /
// autoRepair) go to different API endpoints; the handler sequences them.
type GKENodePoolEditRequestMsg struct {
	ProjectID, Location, ClusterName, PoolName string
	Fields                                     *gcp.NodePoolEdit
	Management                                 *gcp.NodePoolManagement
}

// GKENodePoolEditCanceledMsg is emitted on Esc from the form / diff view.
type GKENodePoolEditCanceledMsg struct{}

// GKENodePoolEditResultMsg carries the outcome of the (possibly multi-step)
// pool edit.
type GKENodePoolEditResultMsg struct {
	TaskID, ClusterName, PoolName string
	Error                         error
}
