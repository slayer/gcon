package errors

import "errors"

// Sentinel errors for common UI operations
var (
	ErrClientNotInitialized        = errors.New("compute client not initialized")
	ErrStorageClientNotInitialized = errors.New("storage client not initialized")
	ErrSQLClientNotInitialized     = errors.New("sql client not initialized")
	ErrIAMClientNotInitialized     = errors.New("iam client not initialized")
	ErrDetailsNotAvailable         = errors.New("details not available")
	ErrSSHNotImplemented           = errors.New("ssh action not yet implemented")
	ErrUnsupportedOS               = errors.New("unsupported operating system")
	ErrFolderEmpty                 = errors.New("folder is empty")
)
