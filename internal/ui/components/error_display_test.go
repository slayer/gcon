package components

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestRenderError_NilError(t *testing.T) {
	result := RenderError(nil)
	assert.Empty(t, result)
}

func TestRenderError_NonGCPError(t *testing.T) {
	err := errors.New("some generic error") //nolint:err113 // Test error
	result := RenderError(err)

	assert.Contains(t, result, "some generic error")
	assert.Contains(t, result, "Press 'r' to retry")
}

func TestRenderError_GCPErrorAllFields(t *testing.T) {
	gcpErr := &gcp.GCPError{
		Code:      gcp.ErrorPermissionDenied,
		Message:   "Permission denied",
		Hint:      "Check IAM permissions",
		Operation: "list instances",
		Resource:  "my-project",
	}

	result := RenderError(gcpErr)

	assert.Contains(t, result, "Permission denied")
	assert.Contains(t, result, "Check IAM permissions")
	assert.Contains(t, result, "Operation: list instances")
	assert.Contains(t, result, "(my-project)")
	assert.Contains(t, result, "Press 'r' to retry")
	assert.Contains(t, result, "🚫") // permission denied icon
}

func TestRenderError_GCPErrorEmptyOperation(t *testing.T) {
	gcpErr := &gcp.GCPError{
		Code:    gcp.ErrorNetwork,
		Message: "Connection error",
		Hint:    "Check your internet",
	}

	result := RenderError(gcpErr)

	assert.Contains(t, result, "Connection error")
	assert.Contains(t, result, "Check your internet")
	// Should not contain "Operation:" since it's empty
	assert.NotContains(t, result, "Operation:")
}

func TestRenderError_GCPErrorEmptyResource(t *testing.T) {
	gcpErr := &gcp.GCPError{
		Code:      gcp.ErrorNotFound,
		Message:   "Resource not found",
		Hint:      "Verify resource exists",
		Operation: "get instance",
		Resource:  "", // Empty resource
	}

	result := RenderError(gcpErr)

	assert.Contains(t, result, "Operation: get instance")
	// Should not contain parentheses for empty resource
	assert.NotContains(t, result, "()")
}

func TestRenderError_WrappedGCPError(t *testing.T) {
	// Test that errors.As works with wrapped errors
	gcpErr := &gcp.GCPError{
		Code:    gcp.ErrorUnauthenticated,
		Message: "Authentication required",
		Hint:    "Run gcloud auth",
	}
	wrappedErr := fmt.Errorf("API call failed: %w", gcpErr)

	result := RenderError(wrappedErr)

	// Should still render GCP error properly due to errors.As
	assert.Contains(t, result, "Authentication required")
	assert.Contains(t, result, "Run gcloud auth")
	assert.Contains(t, result, "🔑") // auth icon
}

func TestRenderError_DuplicateRetryHint(t *testing.T) {
	// Verify that hints containing "(press 'r' to retry)" don't duplicate the retry instruction
	gcpErr := &gcp.GCPError{
		Code:    gcp.ErrorRateLimited,
		Message: "Too many requests",
		Hint:    "Wait a moment and try again (press 'r' to retry)",
	}

	result := RenderError(gcpErr)

	// Count occurrences of retry hint
	retryCount := strings.Count(strings.ToLower(result), "press 'r' to retry")
	assert.Equal(t, 1, retryCount, "retry hint should appear exactly once")
	// Original hint text should be stripped of the retry part
	assert.Contains(t, result, "Wait a moment and try again")
}

func TestErrorIcon(t *testing.T) {
	tests := []struct {
		code     gcp.ErrorCode
		expected string
	}{
		{gcp.ErrorUnauthenticated, "🔑"},
		{gcp.ErrorPermissionDenied, "🚫"},
		{gcp.ErrorNotFound, "❓"},
		{gcp.ErrorRateLimited, "⏳"},
		{gcp.ErrorQuotaExceeded, "⏳"},
		{gcp.ErrorServiceUnavailable, "🔧"},
		{gcp.ErrorNetwork, "🌐"},
		{gcp.ErrorUnknown, "❌"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("ErrorCode_%d", tt.code), func(t *testing.T) {
			result := errorIcon(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}
