package views

// Snapshot action messages for communication between views and the app

// DeleteSnapshotConfirmedMsg is sent after user confirms snapshot deletion
type DeleteSnapshotConfirmedMsg struct {
	SnapshotName string
}

// SnapshotActionResultMsg is sent after a snapshot action completes
type SnapshotActionResultMsg struct {
	Action  string // "delete"
	Success bool
	Error   error
}
