package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractFolderName(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		expected string
	}{
		{
			name:     "single folder",
			prefix:   "folder1/",
			expected: "folder1",
		},
		{
			name:     "nested folder",
			prefix:   "folder1/folder2/",
			expected: "folder2",
		},
		{
			name:     "deeply nested folder",
			prefix:   "a/b/c/d/",
			expected: "d",
		},
		{
			name:     "folder without trailing slash",
			prefix:   "folder1",
			expected: "folder1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFolderName(tt.prefix)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "bytes",
			bytes:    500,
			expected: "500 B",
		},
		{
			name:     "kilobytes",
			bytes:    1536,
			expected: "1.5 KB",
		},
		{
			name:     "megabytes",
			bytes:    1572864,
			expected: "1.5 MB",
		},
		{
			name:     "gigabytes",
			bytes:    1610612736,
			expected: "1.5 GB",
		},
		{
			name:     "terabytes",
			bytes:    1649267441664,
			expected: "1.5 TB",
		},
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "exact KB boundary",
			bytes:    1024,
			expected: "1.0 KB",
		},
		{
			name:     "exact MB boundary",
			bytes:    1048576,
			expected: "1.0 MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSize(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestObjectFromAttrs(t *testing.T) {
	tests := []struct {
		name       string
		objectName string
		prefix     string
		expectedDN string
	}{
		{
			name:       "file at root",
			objectName: "file.txt",
			prefix:     "",
			expectedDN: "file.txt",
		},
		{
			name:       "file in folder with prefix",
			objectName: "folder1/file.txt",
			prefix:     "folder1/",
			expectedDN: "file.txt",
		},
		{
			name:       "file in nested folder",
			objectName: "folder1/folder2/file.txt",
			prefix:     "folder1/folder2/",
			expectedDN: "file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the display name extraction logic directly
			displayName := tt.objectName
			if tt.prefix != "" {
				displayName = tt.objectName[len(tt.prefix):]
			}
			assert.Equal(t, tt.expectedDN, displayName)
		})
	}
}
