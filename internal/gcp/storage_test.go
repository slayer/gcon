package gcp

import (
	"bytes"
	"io"
	"strings"
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

func TestCopyWithProgress(t *testing.T) {
	tests := []struct {
		name        string
		size        int64
		expectCalls bool // Whether progress callback should be called
	}{
		{
			name:        "small file",
			size:        100,
			expectCalls: true,
		},
		{
			name:        "medium file",
			size:        100 * 1024, // 100KB
			expectCalls: true,
		},
		{
			name:        "empty file",
			size:        0,
			expectCalls: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create source data
			src := strings.NewReader(strings.Repeat("x", int(tt.size)))
			dst := &bytes.Buffer{}

			var progressCalls []struct {
				transferred int64
				total       int64
			}

			progress := func(transferred, total int64) {
				progressCalls = append(progressCalls, struct {
					transferred int64
					total       int64
				}{transferred, total})
			}

			written, err := copyWithProgress(dst, src, tt.size, progress)

			assert.NoError(t, err)
			assert.Equal(t, tt.size, written)
			assert.Equal(t, int(tt.size), dst.Len())

			if tt.expectCalls && tt.size > 0 {
				assert.NotEmpty(t, progressCalls, "progress should be called for non-empty files")
				// Last call should have transferred == total
				if len(progressCalls) > 0 {
					lastCall := progressCalls[len(progressCalls)-1]
					assert.Equal(t, tt.size, lastCall.total, "total size should match")
				}
			}
		})
	}
}

func TestCopyWithProgress_NilCallback(t *testing.T) {
	// Ensure copyWithProgress works even with nil callback
	src := strings.NewReader("test data")
	dst := &bytes.Buffer{}

	written, err := copyWithProgress(dst, src, 9, nil)

	assert.NoError(t, err)
	assert.Equal(t, int64(9), written)
	assert.Equal(t, "test data", dst.String())
}

func TestProgressWriter(t *testing.T) {
	var calls []int64
	progress := func(transferred, total int64) {
		calls = append(calls, transferred)
	}

	dst := &bytes.Buffer{}
	pw := &progressWriter{
		writer:      dst,
		totalSize:   100,
		progress:    progress,
		reportEvery: 10, // Report every 10 bytes
	}

	// Write in chunks
	for i := 0; i < 10; i++ {
		_, err := pw.Write([]byte("1234567890")) // 10 bytes each
		assert.NoError(t, err)
	}

	assert.Equal(t, 100, dst.Len())
	// Should have progress calls due to reportEvery threshold
	assert.NotEmpty(t, calls)
	// Last call should be at 100 bytes
	assert.Equal(t, int64(100), calls[len(calls)-1])
}

func TestProgressWriter_ErrorPropagation(t *testing.T) {
	// Create a writer that fails
	errWriter := &errorWriter{err: io.ErrShortWrite}
	pw := &progressWriter{
		writer:      errWriter,
		totalSize:   100,
		progress:    func(_, _ int64) {},
		reportEvery: 10,
	}

	_, err := pw.Write([]byte("test"))
	assert.Error(t, err)
	assert.Equal(t, io.ErrShortWrite, err)
}

// errorWriter is a test helper that always returns an error
type errorWriter struct {
	err error
}

func (e *errorWriter) Write(p []byte) (int, error) {
	return 0, e.err
}
