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

// CloudRunEditRequestMsg requests opening the edit view for an existing service
type CloudRunEditRequestMsg struct {
	ProjectID   string
	ServiceName string
	FullName    string
}

// CloudRunCreateRequestMsg requests opening the create view for a new service
type CloudRunCreateRequestMsg struct {
	ProjectID string
}

// CloudRunEditResultMsg is the outcome of an edit/create operation
type CloudRunEditResultMsg struct {
	Name    string
	Action  string // "edit" or "create"
	Success bool
	Error   error
}

// CloudRunEditCanceledMsg indicates user canceled editing/creating
type CloudRunEditCanceledMsg struct{}
