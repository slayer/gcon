package debug

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnableDebug(t *testing.T) {
	// Clean up any existing debug log
	defer func() {
		_ = Close()                            //nolint:errcheck // Test cleanup
		_ = os.Remove("gcon-debug.log")        //nolint:errcheck // Test cleanup
	}()

	// Initially should be disabled
	assert.False(t, IsEnabled())

	// Enable debug logging
	err := EnableDebug()
	require.NoError(t, err)
	assert.True(t, IsEnabled())

	// Check that file was created
	_, err = os.Stat("gcon-debug.log")
	require.NoError(t, err)
}

func TestLog(t *testing.T) {
	// Clean up any existing debug log
	defer func() {
		_ = Close()                            //nolint:errcheck // Test cleanup
		_ = os.Remove("gcon-debug.log")        //nolint:errcheck // Test cleanup
	}()

	// Enable debug logging
	err := EnableDebug()
	require.NoError(t, err)

	// Write a log message
	Log("test message: %s", "value")

	// Close the file to flush
	err = Close()
	require.NoError(t, err)

	// Read the log file
	content, err := os.ReadFile("gcon-debug.log")
	require.NoError(t, err)
	assert.Contains(t, string(content), "test message: value")
}

func TestLogWhenDisabled(t *testing.T) {
	// Ensure debug is disabled
	_ = Close() //nolint:errcheck // Test cleanup

	// This should not panic or create a file
	Log("test message")

	// Check that file was not created
	_, err := os.Stat("gcon-debug.log")
	assert.True(t, os.IsNotExist(err))
}

func TestLogView(t *testing.T) {
	// Clean up any existing debug log
	defer func() {
		_ = Close()                            //nolint:errcheck // Test cleanup
		_ = os.Remove("gcon-debug.log")        //nolint:errcheck // Test cleanup
	}()

	// Enable debug logging
	err := EnableDebug()
	require.NoError(t, err)

	// Log a view
	content := "line1\nline2\nline3"
	LogView("TestView", content)

	// Close the file to flush
	err = Close()
	require.NoError(t, err)

	// Read the log file
	logContent, err := os.ReadFile("gcon-debug.log")
	require.NoError(t, err)

	logStr := string(logContent)
	assert.Contains(t, logStr, "TestView:")
	assert.Contains(t, logStr, "height=3")
	assert.Contains(t, logStr, "first")
	assert.Contains(t, logStr, "last")
}

func TestClose(t *testing.T) {
	// Clean up any existing debug log
	defer func() {
		_ = os.Remove("gcon-debug.log") //nolint:errcheck // Test cleanup
	}()

	// Enable debug logging
	err := EnableDebug()
	require.NoError(t, err)
	assert.True(t, IsEnabled())

	// Close should disable and close file
	err = Close()
	require.NoError(t, err)
	assert.False(t, IsEnabled())

	// Calling Close again should not error
	err = Close()
	require.NoError(t, err)
}

func TestTruncStr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			max:      10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			max:      5,
			expected: "hello",
		},
		{
			name:     "long string",
			input:    "hello world this is a long string",
			max:      10,
			expected: "hello worl...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncStr(tt.input, tt.max)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMultipleLogCalls(t *testing.T) {
	// Clean up any existing debug log
	defer func() {
		_ = Close()                            //nolint:errcheck // Test cleanup
		_ = os.Remove("gcon-debug.log")        //nolint:errcheck // Test cleanup
	}()

	// Enable debug logging
	err := EnableDebug()
	require.NoError(t, err)

	// Write multiple log messages
	Log("message 1")
	Log("message 2")
	Log("message 3")

	// Close the file to flush
	err = Close()
	require.NoError(t, err)

	// Read the log file
	content, err := os.ReadFile("gcon-debug.log")
	require.NoError(t, err)

	logStr := string(content)
	assert.Contains(t, logStr, "message 1")
	assert.Contains(t, logStr, "message 2")
	assert.Contains(t, logStr, "message 3")

	// Messages should be on separate lines
	lines := strings.Split(strings.TrimSpace(logStr), "\n")
	assert.GreaterOrEqual(t, len(lines), 3)
}
