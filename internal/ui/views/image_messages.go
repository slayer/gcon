package views

// Image action messages for communication between views and the app

// DeleteImageConfirmedMsg is sent after user confirms image deletion
type DeleteImageConfirmedMsg struct {
	ImageName string
}

// ImageActionResultMsg is sent after an image action completes
type ImageActionResultMsg struct {
	Action  string // "delete"
	Success bool
	Error   error
}
