package views

import "github.com/slayer/gcon/internal/gcp"

// CloudRunServiceSelectedMsg is sent when a Cloud Run service is selected from the list
type CloudRunServiceSelectedMsg struct {
	Service gcp.CloudRunService
}

// DeleteCloudRunServiceConfirmedMsg is sent when user confirms Cloud Run service deletion
type DeleteCloudRunServiceConfirmedMsg struct {
	FullName string
	Name     string
}

// CloudRunServiceActionResultMsg reports the result of an async Cloud Run operation
type CloudRunServiceActionResultMsg struct {
	Name    string
	Action  string // "delete", "update_traffic"
	Success bool
	Error   error
}

// CloudRunTrafficUpdateMsg is sent when user submits a traffic split update
type CloudRunTrafficUpdateMsg struct {
	FullName string
	Targets  []gcp.CloudRunTrafficTarget
}
