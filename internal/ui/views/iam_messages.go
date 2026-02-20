package views

import "github.com/slayer/gcon/internal/gcp"

// ServiceAccountSelectedMsg is sent when a service account is selected from the list
type ServiceAccountSelectedMsg struct {
	ServiceAccount gcp.ServiceAccount
}

// ServiceAccountCreateRequestMsg is sent when user wants to create a new service account
type ServiceAccountCreateRequestMsg struct {
	ProjectID string
}

// ServiceAccountCreateCanceledMsg is sent when user cancels service account creation
type ServiceAccountCreateCanceledMsg struct{}

// CreateServiceAccountMsg is sent when the create form is submitted
type CreateServiceAccountMsg struct {
	ProjectID   string
	AccountID   string
	DisplayName string
	Description string
}

// DeleteServiceAccountConfirmedMsg is sent when user confirms service account deletion
type DeleteServiceAccountConfirmedMsg struct {
	Email string
}

// ToggleServiceAccountMsg is sent when user enables/disables a service account
type ToggleServiceAccountMsg struct {
	Email   string
	Disable bool
}

// ServiceAccountActionResultMsg reports the result of an async service account operation
type ServiceAccountActionResultMsg struct {
	Action  string // "create", "delete", "enable", "disable"
	Success bool
	Error   error
}

// CreateServiceAccountKeyMsg is sent when user requests a new key
type CreateServiceAccountKeyMsg struct {
	Email string
}

// DeleteServiceAccountKeyMsg is sent when user confirms key deletion
type DeleteServiceAccountKeyMsg struct {
	KeyName string
	Email   string
}

// DownloadServiceAccountKeyMsg is sent when user explicitly downloads a pending key
type DownloadServiceAccountKeyMsg struct {
	KeyJSON []byte
	KeyID   string
}

// ServiceAccountKeyActionResultMsg reports the result of a key operation
type ServiceAccountKeyActionResultMsg struct {
	Action  string // "create_key", "delete_key"
	Success bool
	Error   error
	KeyJSON []byte // Only set on successful create_key — one-time download
	KeyID   string
}

// ViewIAMPolicyRequestMsg is sent when user navigates to IAM policy view
type ViewIAMPolicyRequestMsg struct {
	ProjectID string
}

// ViewCustomRolesRequestMsg is sent when user navigates to custom roles view
type ViewCustomRolesRequestMsg struct {
	ProjectID string
}

// CustomRoleSelectedMsg is sent when a custom role is selected from the list
type CustomRoleSelectedMsg struct {
	Role gcp.CustomRole
}
