package views

import "github.com/slayer/gcon/internal/gcp"

// SubnetSelectedMsg is sent when a subnet is selected from the list or network details
type SubnetSelectedMsg struct {
	SubnetName string
	Region     string
}

// SubnetCreateRequestMsg is sent to open the subnet creation form
type SubnetCreateRequestMsg struct{}

// SubnetCreateCanceledMsg is sent when user cancels subnet creation
type SubnetCreateCanceledMsg struct{}

// CreateSubnetMsg carries the form data to create a subnet
type CreateSubnetMsg struct {
	Config gcp.SubnetCreateConfig
}

// DeleteSubnetConfirmedMsg is sent when user confirms subnet deletion
type DeleteSubnetConfirmedMsg struct {
	SubnetName string
	Region     string
}

// SubnetActionResultMsg reports the result of an async subnet operation
type SubnetActionResultMsg struct {
	Action  string // "delete", "create"
	Success bool
	Error   error
}
