package views

import "github.com/slayer/gcon/internal/gcp"

// LoadBalancerSelectedMsg is emitted by the list view when the user presses
// Enter on a row. The app routes it to ViewLoadBalancerDetails.
type LoadBalancerSelectedMsg struct {
	SelfLink string
	Scope    string
	Name     string
}

// LoadBalancerDeleteRequestMsg is emitted by the details view when the user
// confirms a delete. The app runs the cascade against the GCP client and
// replies with LoadBalancerDeletedMsg.
type LoadBalancerDeleteRequestMsg struct {
	Cascade Cascade
}

// LoadBalancerDeletedMsg is emitted by the app after the delete cascade
// completes (or partially completes). Errs maps each failed CascadeItem
// (keyed by its URL) to the error that occurred.
type LoadBalancerDeletedMsg struct {
	Errs map[string]error
}

// Internal messages for backend health fan-out. Lowercase by convention
// — these never cross the view boundary.
type lbGroupHealthLoadedMsg struct {
	groupURL string
	statuses []gcp.InstanceHealth
}

type lbGroupHealthErrorMsg struct {
	groupURL string
	err      error
}

type lbGroupSkippedMsg struct {
	groupURL string
	reason   string // human-readable, e.g. "Cloud Storage backend"
}

// lbHealthRefreshMsg is emitted on the auto-refresh tick to re-run the
// backend-health fan-out independently of the metrics refresh.
type lbHealthRefreshMsg struct{}
