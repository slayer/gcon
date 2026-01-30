package views

// Instance action messages for communication between views and the app

// DeleteInstanceConfirmedMsg is sent after user confirms instance deletion
type DeleteInstanceConfirmedMsg struct {
	InstanceName string
	Zone         string
}

// InstanceActionResultMsg is sent after an instance action completes
type InstanceActionResultMsg struct {
	Action  string // "delete"
	Success bool
	Error   error
}
