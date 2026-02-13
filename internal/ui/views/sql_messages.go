package views

import "github.com/slayer/gcon/internal/gcp"

// SQLInstanceSelectedMsg is sent when a SQL instance is selected from the list
type SQLInstanceSelectedMsg struct {
	Instance gcp.SQLInstance
}

// SQLInstanceActionMsg is sent when user requests a lifecycle action on a SQL instance
type SQLInstanceActionMsg struct {
	InstanceName string
	Action       string // "start", "stop", "restart"
}

// DeleteSQLInstanceConfirmedMsg is sent when user confirms SQL instance deletion
type DeleteSQLInstanceConfirmedMsg struct {
	InstanceName string
}

// SQLInstanceActionResultMsg reports the result of an async SQL instance operation
type SQLInstanceActionResultMsg struct {
	InstanceName string
	Action       string // "start", "stop", "restart", "delete"
	Success      bool
	Error        error
}

// CreateSQLBackupMsg is sent when user requests an on-demand backup
type CreateSQLBackupMsg struct {
	InstanceName string
	Description  string
}

// SQLBackupActionResultMsg reports the result of a backup operation
type SQLBackupActionResultMsg struct {
	Action  string // "create_backup"
	Success bool
	Error   error
}
