package errors

import "errors"

// Sentinel errors for common UI operations
var (
	ErrClientNotInitialized         = errors.New("compute client not initialized")
	ErrStorageClientNotInitialized  = errors.New("storage client not initialized")
	ErrSQLClientNotInitialized      = errors.New("sql client not initialized")
	ErrIAMClientNotInitialized      = errors.New("iam client not initialized")
	ErrCloudRunClientNotInitialized = errors.New("cloud run client not initialized")
	ErrGCPClientNotInitialized      = errors.New("GCP client not initialized")
	ErrGKEClientNotInitialized      = errors.New("GKE container client not initialized")
	ErrDetailsNotAvailable          = errors.New("details not available")
	ErrUnsupportedOS                = errors.New("unsupported operating system")
	ErrFolderEmpty                  = errors.New("folder is empty")
	ErrPartialConfigEditFailed      = errors.New("some configuration changes failed")
)
