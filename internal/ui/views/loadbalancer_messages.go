package views

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
