package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateEmail_ShortEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		maxWidth int
		expected string
	}{
		{
			name:     "email fits exactly",
			email:    "user@example.com",
			maxWidth: 16,
			expected: "user@example.com",
		},
		{
			name:     "email fits with room",
			email:    "john@test.com",
			maxWidth: 25,
			expected: "john@test.com",
		},
		{
			name:     "short email",
			email:    "a@b.c",
			maxWidth: 10,
			expected: "a@b.c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateEmail(tt.email, tt.maxWidth)
			assert.Equal(t, tt.expected, result)
			assert.LessOrEqual(t, len(result), tt.maxWidth, "Result should not exceed maxWidth")
		})
	}
}

func TestTruncateEmail_LongEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		maxWidth int
	}{
		{
			name:     "long service account email",
			email:    "very-long-service-account@my-project.iam.gserviceaccount.com",
			maxWidth: 25,
		},
		{
			name:     "long user email",
			email:    "john.doe.longname@verylongdomain.example.com",
			maxWidth: 25,
		},
		{
			name:     "extremely long email with tiny width",
			email:    "superlongemailaddress@verylongdomain.com",
			maxWidth: 15,
		},
		{
			name:     "long email with balanced truncation",
			email:    "myserviceaccount@project-123.iam.gserviceaccount.com",
			maxWidth: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateEmail(tt.email, tt.maxWidth)
			assert.LessOrEqual(t, len(result), tt.maxWidth, "Result should not exceed maxWidth")
			assert.Contains(t, result, "...", "Truncated email should contain ellipsis")
			// Verify it contains parts of the original email
			assert.True(t, strings.HasPrefix(result, string(tt.email[0])), "Should preserve start")
			t.Logf("Original: %s (len=%d)", tt.email, len(tt.email))
			t.Logf("Truncated: %s (len=%d)", result, len(result))
		})
	}
}

func TestTruncateEmail_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		maxWidth int
		expected string
	}{
		{
			name:     "empty string",
			email:    "",
			maxWidth: 10,
			expected: "",
		},
		{
			name:     "email without @ (malformed)",
			email:    "notanemail",
			maxWidth: 5,
			expected: "notan",
		},
		{
			name:     "very small maxWidth",
			email:    "user@example.com",
			maxWidth: 5,
			expected: "user@",
		},
		{
			name:     "maxWidth less than 10",
			email:    "longuser@example.com",
			maxWidth: 8,
			expected: "longuser",
		},
		{
			name:     "email with multiple @ symbols (malformed)",
			email:    "user@@example.com",
			maxWidth: 12,
			expected: "user@@exa...",
		},
		{
			name:     "single character parts",
			email:    "a@b",
			maxWidth: 5,
			expected: "a@b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateEmail(tt.email, tt.maxWidth)
			assert.Equal(t, tt.expected, result)
			assert.LessOrEqual(t, len(result), tt.maxWidth, "Result should not exceed maxWidth")
		})
	}
}

func TestTruncateEmail_PreservesStartAndDomain(t *testing.T) {
	email := "myserviceaccount@project-123.iam.gserviceaccount.com"
	maxWidth := 25

	result := truncateEmail(email, maxWidth)

	// Should preserve beginning of username
	assert.True(t, result[0] == 'm', "Should preserve start of username")

	// Should preserve end of domain
	assert.True(t, result[len(result)-4:] == ".com", "Should preserve domain extension")

	// Should contain ellipsis
	assert.Contains(t, result, "...", "Should contain ellipsis")

	// Should not exceed max width
	assert.LessOrEqual(t, len(result), maxWidth)
}

func TestTruncateEmail_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		maxWidth int
	}{
		{
			name:     "Google Cloud service account",
			email:    "gcon-app@my-gcp-project-12345.iam.gserviceaccount.com",
			maxWidth: 25,
		},
		{
			name:     "Gmail user",
			email:    "john.doe.developer@gmail.com",
			maxWidth: 25,
		},
		{
			name:     "Corporate email",
			email:    "firstname.lastname@company.example.com",
			maxWidth: 25,
		},
		{
			name:     "Short organizational email",
			email:    "admin@org.io",
			maxWidth: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateEmail(tt.email, tt.maxWidth)

			// Basic validations
			assert.LessOrEqual(t, len(result), tt.maxWidth, "Should not exceed maxWidth")
			assert.NotEmpty(t, result, "Should not be empty")

			// If truncated, should have ellipsis
			if len(tt.email) > tt.maxWidth {
				assert.Contains(t, result, "...", "Long email should be truncated with ellipsis")
			}

			t.Logf("Original: %s", tt.email)
			t.Logf("Truncated: %s (len=%d)", result, len(result))
		})
	}
}

func TestTruncateEmail_ConsistentLength(t *testing.T) {
	// Test that all truncated emails stay within maxWidth
	emails := []string{
		"a@b.c",
		"user@example.com",
		"very-long-username@verylongdomain.example.com",
		"sa@project-id.iam.gserviceaccount.com",
		"super-duper-extremely-long-service-account-name@my-very-long-project-identifier.iam.gserviceaccount.com",
	}

	maxWidths := []int{10, 15, 20, 25, 30}

	for _, email := range emails {
		for _, maxWidth := range maxWidths {
			result := truncateEmail(email, maxWidth)
			assert.LessOrEqual(t, len(result), maxWidth,
				"Email '%s' truncated to width %d resulted in '%s' (len=%d)",
				email, maxWidth, result, len(result))
		}
	}
}
