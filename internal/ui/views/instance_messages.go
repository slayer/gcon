package views

import "github.com/slayer/gcon/internal/gcp"

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

// --- Instance Create messages ---

// InstanceCreateRequestMsg requests opening the create view
type InstanceCreateRequestMsg struct {
	ProjectID string
}

// CreateInstanceMsg carries the config to actually create the instance via the GCP API
type CreateInstanceMsg struct {
	Config gcp.InstanceCreateConfig
}

// InstanceCreateResultMsg reports the outcome of the create operation
type InstanceCreateResultMsg struct {
	Name    string
	Success bool
	Error   error
}

// InstanceCreateCanceledMsg indicates user canceled creation
type InstanceCreateCanceledMsg struct{}

// --- Instance Config Edit messages ---

// InstanceConfigEditRequestMsg requests opening the config edit view.
// Distinct from InstanceEditRequestMsg which handles labels/tags editing.
type InstanceConfigEditRequestMsg struct {
	ProjectID    string
	InstanceName string
	Zone         string
}

// InstanceConfigEditChange represents a single change to apply during config edit
type InstanceConfigEditChange struct {
	Field    string // "machine_type", "disk_size"
	OldValue string
	NewValue string
}

// InstanceConfigEditSubmitMsg carries the changes to apply to the instance
type InstanceConfigEditSubmitMsg struct {
	ProjectID    string
	InstanceName string
	Zone         string
	BootDiskName string // actual boot disk name (may differ from instance name)
	Changes      []InstanceConfigEditChange
}

// InstanceConfigEditResultMsg reports the outcome of the config edit
type InstanceConfigEditResultMsg struct {
	Action        string // "config_edit"
	Success       bool
	Error         error
	PartialErrors []string // individual step failures
}

// InstanceConfigEditCanceledMsg indicates user canceled config editing
type InstanceConfigEditCanceledMsg struct{}
