package gcp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"
)

func TestParseError_APIErrors(t *testing.T) {
	tests := []struct {
		name             string
		httpCode         int
		apiMessage       string
		operation        string
		resource         string
		expectedCode     ErrorCode
		expectedMsg      string
		wantHintContains string
	}{
		{
			name:             "401 unauthenticated",
			httpCode:         401,
			apiMessage:       "Request had invalid authentication credentials",
			operation:        "list projects",
			resource:         "",
			expectedCode:     ErrorUnauthenticated,
			expectedMsg:      "Authentication required",
			wantHintContains: "gcloud auth application-default login",
		},
		{
			name:             "403 permission denied",
			httpCode:         403,
			apiMessage:       "Caller does not have permission",
			operation:        "list instances",
			resource:         "my-project",
			expectedCode:     ErrorPermissionDenied,
			expectedMsg:      "Permission denied",
			wantHintContains: "IAM permissions",
		},
		{
			name:             "403 quota exceeded",
			httpCode:         403,
			apiMessage:       "Quota exceeded for quota metric",
			operation:        "start instance",
			resource:         "my-instance",
			expectedCode:     ErrorQuotaExceeded,
			expectedMsg:      "Quota exceeded",
			wantHintContains: "quota increase",
		},
		{
			name:             "404 not found",
			httpCode:         404,
			apiMessage:       "The resource was not found",
			operation:        "get instance",
			resource:         "my-instance",
			expectedCode:     ErrorNotFound,
			expectedMsg:      "Resource not found",
			wantHintContains: "Verify the resource exists",
		},
		{
			name:             "429 rate limited",
			httpCode:         429,
			apiMessage:       "Rate Limit Exceeded",
			operation:        "list instances",
			resource:         "my-project",
			expectedCode:     ErrorRateLimited,
			expectedMsg:      "Too many requests",
			wantHintContains: "Wait",
		},
		{
			name:             "503 service unavailable",
			httpCode:         503,
			apiMessage:       "Service unavailable",
			operation:        "list projects",
			resource:         "",
			expectedCode:     ErrorServiceUnavailable,
			expectedMsg:      "Service temporarily unavailable",
			wantHintContains: "Try again later",
		},
		{
			name:             "unknown error code",
			httpCode:         500,
			apiMessage:       "Internal server error",
			operation:        "stop instance",
			resource:         "my-vm",
			expectedCode:     ErrorUnknown,
			expectedMsg:      "API error",
			wantHintContains: "retry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := &googleapi.Error{
				Code:    tt.httpCode,
				Message: tt.apiMessage,
			}

			result := ParseError(apiErr, tt.operation, tt.resource)

			require.NotNil(t, result)
			assert.Equal(t, tt.expectedCode, result.Code)
			assert.Equal(t, tt.expectedMsg, result.Message)
			assert.Contains(t, result.Hint, tt.wantHintContains)
			assert.Equal(t, tt.operation, result.Operation)
			assert.Equal(t, tt.resource, result.Resource)
			assert.Equal(t, tt.httpCode, result.HTTPCode)
		})
	}
}

func TestParseError_NetworkErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode ErrorCode
	}{
		{
			name:     "connection error",
			err:      errors.New("dial tcp: connection refused"),
			wantCode: ErrorNetwork,
		},
		{
			name:     "timeout error",
			err:      errors.New("context deadline exceeded (timeout)"),
			wantCode: ErrorNetwork,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseError(tt.err, "test", "resource")

			require.NotNil(t, result)
			assert.Equal(t, tt.wantCode, result.Code)
			assert.Equal(t, "Connection error", result.Message)
		})
	}
}

func TestParseError_NilError(t *testing.T) {
	result := ParseError(nil, "test", "resource")
	assert.Nil(t, result)
}

func TestGCPError_ErrorChainPreserved(t *testing.T) {
	// Verify that errors.As can extract the original googleapi.Error
	originalErr := &googleapi.Error{Code: 403, Message: "forbidden"}
	gcpErr := ParseError(originalErr, "test", "resource")

	var apiErr *googleapi.Error
	assert.True(t, errors.As(gcpErr, &apiErr), "should be able to extract original googleapi.Error")
	assert.Equal(t, 403, apiErr.Code)
}

func TestGCPError_ErrorMethod(t *testing.T) {
	gcpErr := &GCPError{
		Message: "Permission denied",
	}
	assert.Equal(t, "Permission denied", gcpErr.Error())
}

func TestWrapListError(t *testing.T) {
	t.Run("wraps error with context", func(t *testing.T) {
		apiErr := &googleapi.Error{Code: 403, Message: "forbidden"}
		result := WrapListError(apiErr, "instances", "my-project")

		gcpErr := &GCPError{}
		ok := errors.As(result, &gcpErr)
		require.True(t, ok)
		assert.Equal(t, "list instances", gcpErr.Operation)
		assert.Equal(t, "my-project", gcpErr.Resource)
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		result := WrapListError(nil, "instances", "my-project")
		assert.Nil(t, result)
	})
}

func TestWrapGetError(t *testing.T) {
	t.Run("wraps error with context", func(t *testing.T) {
		apiErr := &googleapi.Error{Code: 404, Message: "not found"}
		result := WrapGetError(apiErr, "instance", "my-vm")

		gcpErr := &GCPError{}
		ok := errors.As(result, &gcpErr)
		require.True(t, ok)
		assert.Equal(t, "get instance", gcpErr.Operation)
		assert.Equal(t, "my-vm", gcpErr.Resource)
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		result := WrapGetError(nil, "instance", "my-vm")
		assert.Nil(t, result)
	})
}

func TestWrapActionError(t *testing.T) {
	t.Run("wraps error with context", func(t *testing.T) {
		apiErr := &googleapi.Error{Code: 403, Message: "forbidden"}
		result := WrapActionError(apiErr, "start instance", "my-vm")

		gcpErr := &GCPError{}
		ok := errors.As(result, &gcpErr)
		require.True(t, ok)
		assert.Equal(t, "start instance", gcpErr.Operation)
		assert.Equal(t, "my-vm", gcpErr.Resource)
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		result := WrapActionError(nil, "start instance", "my-vm")
		assert.Nil(t, result)
	})
}

func TestContainsQuotaMessage(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"Quota exceeded for quota metric", true},
		{"Limit exceeded", true},
		{"rate limit reached", true},
		{"Permission denied", false},
		{"Not found", false},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			assert.Equal(t, tt.want, containsQuotaMessage(tt.msg))
		})
	}
}
