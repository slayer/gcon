package gcp

import (
	"errors"
	"strings"

	"google.golang.org/api/googleapi"
)

// ErrorCode represents categorized GCP error types
type ErrorCode int

const (
	ErrorUnknown            ErrorCode = iota
	ErrorUnauthenticated              // 401 - credentials invalid or expired
	ErrorPermissionDenied             // 403 - missing IAM permissions
	ErrorNotFound                     // 404 - resource doesn't exist
	ErrorRateLimited                  // 429 - too many requests
	ErrorQuotaExceeded                // 429 with quota message
	ErrorServiceUnavailable           // 503 - GCP service issues
	ErrorNetwork                      // connection/network errors
)

// GCPError wraps GCP API errors with user-friendly information
type GCPError struct {
	Code       ErrorCode
	Message    string // User-friendly message
	Hint       string // Actionable hint for the user
	Operation  string // What operation was attempted (e.g., "list instances")
	Resource   string // Resource being accessed (e.g., project ID, instance name)
	Underlying error  // Original error for error chaining
	HTTPCode   int    // Original HTTP status code (0 if not applicable)
}

func (e *GCPError) Error() string {
	return e.Message
}

func (e *GCPError) Unwrap() error {
	return e.Underlying
}

// ParseError extracts meaningful information from GCP API errors
func ParseError(err error, operation, resource string) *GCPError {
	if err == nil {
		return nil
	}

	gcpErr := &GCPError{
		Code:       ErrorUnknown,
		Operation:  operation,
		Resource:   resource,
		Underlying: err,
	}

	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		gcpErr.HTTPCode = apiErr.Code
		gcpErr.Code, gcpErr.Message, gcpErr.Hint = classifyAPIError(apiErr)
		return gcpErr
	}

	// Handle non-API errors (network issues, timeouts, etc.)
	errMsg := err.Error()
	if strings.Contains(errMsg, "connection") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "dial") {
		gcpErr.Code = ErrorNetwork
		gcpErr.Message = "Connection error"
		gcpErr.Hint = "Check your internet connection and try again"
		return gcpErr
	}

	// Generic fallback
	gcpErr.Message = "An error occurred"
	gcpErr.Hint = "Press 'r' to retry"
	return gcpErr
}

// classifyAPIError maps HTTP status codes to user-friendly messages
func classifyAPIError(apiErr *googleapi.Error) (ErrorCode, string, string) {
	switch apiErr.Code {
	case 401:
		return ErrorUnauthenticated,
			"Authentication required",
			"Run 'gcloud auth application-default login' to authenticate"

	case 403:
		// Distinguish quota errors from permission errors
		if containsQuotaMessage(apiErr.Message) {
			return ErrorQuotaExceeded,
				"Quota exceeded",
				"Wait and retry, or request a quota increase in Cloud Console"
		}
		return ErrorPermissionDenied,
			"Permission denied",
			"Ensure your account has the required IAM permissions for this resource"

	case 404:
		return ErrorNotFound,
			"Resource not found",
			"Verify the resource exists and the name/ID is correct"

	case 429:
		return ErrorRateLimited,
			"Too many requests",
			"Wait a moment and try again (press 'r' to retry)"

	case 503:
		return ErrorServiceUnavailable,
			"Service temporarily unavailable",
			"GCP service is experiencing issues. Try again later"

	default:
		return ErrorUnknown,
			"API error",
			"Press 'r' to retry"
	}
}

// containsQuotaMessage checks if the error message indicates a quota issue
func containsQuotaMessage(msg string) bool {
	msgLower := strings.ToLower(msg)
	keywords := []string{"quota", "limit exceeded", "rate limit"}
	for _, kw := range keywords {
		if strings.Contains(msgLower, kw) {
			return true
		}
	}
	return false
}

// WrapListError wraps errors from list operations with context
func WrapListError(err error, resourceType, scope string) error {
	if err == nil {
		return nil
	}
	return ParseError(err, "list "+resourceType, scope)
}

// WrapGetError wraps errors from get operations with context
func WrapGetError(err error, resourceType, name string) error {
	if err == nil {
		return nil
	}
	return ParseError(err, "get "+resourceType, name)
}

// WrapActionError wraps errors from action operations (start/stop/reset) with context
func WrapActionError(err error, action, resourceName string) error {
	if err == nil {
		return nil
	}
	return ParseError(err, action, resourceName)
}
