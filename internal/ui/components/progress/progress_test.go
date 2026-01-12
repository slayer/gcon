package progress

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	p := New()

	assert.NotNil(t, p)
	assert.Equal(t, 50, p.width)
}

func TestSetSize(t *testing.T) {
	p := New()

	t.Run("sets width when greater than 20", func(t *testing.T) {
		p.SetSize(100)
		assert.Equal(t, 90, p.width)
	})

	t.Run("ignores width when 20 or less", func(t *testing.T) {
		p.SetSize(20)
		assert.Equal(t, 90, p.width) // Should remain unchanged
	})
}

func TestSetProgress(t *testing.T) {
	p := New()

	p.SetProgress("Downloading", "test.txt", 1, 5, 500, 1000)

	assert.Equal(t, "Downloading", p.Title)
	assert.Equal(t, "test.txt", p.CurrentFile)
	assert.Equal(t, 1, p.CurrentFileNum)
	assert.Equal(t, 5, p.TotalFiles)
	assert.Equal(t, int64(500), p.BytesTransferred)
	assert.Equal(t, int64(1000), p.TotalBytes)
}

func TestView(t *testing.T) {
	p := New()
	p.SetSize(60)

	t.Run("single file progress", func(t *testing.T) {
		p.SetProgress("Downloading", "file.txt", 1, 1, 500, 1000)

		view := p.View()

		// Should contain the title
		assert.Contains(t, view, "Downloading")
		// Should contain the filename
		assert.Contains(t, view, "file.txt")
		// Should contain percentage
		assert.Contains(t, view, "50%")
		// Should contain progress bar characters
		assert.True(t, strings.Contains(view, "█") || strings.Contains(view, "░"))
	})

	t.Run("multi-file progress shows count", func(t *testing.T) {
		p.SetProgress("Uploading", "doc.pdf", 2, 5, 1000, 5000)

		view := p.View()

		// Should show "2 of 5 files"
		assert.Contains(t, view, "2 of 5 files")
		assert.Contains(t, view, "doc.pdf")
	})

	t.Run("truncates long filename", func(t *testing.T) {
		longName := "this_is_a_very_long_filename_that_should_be_truncated_for_display.txt"
		p.SetProgress("Downloading", longName, 1, 1, 0, 1000)

		view := p.View()

		// Should contain ellipsis for truncation
		assert.Contains(t, view, "...")
	})

	t.Run("handles zero total bytes", func(t *testing.T) {
		p.SetProgress("Downloading", "empty.txt", 1, 1, 0, 0)

		view := p.View()

		// Should render without panic
		assert.NotEmpty(t, view)
		assert.Contains(t, view, "0%")
	})
}

func TestProgressUpdate(t *testing.T) {
	update := ProgressUpdate{
		BytesTransferred: 1024,
		TotalBytes:       2048,
		CurrentFile:      "test.txt",
		CurrentFileNum:   1,
		TotalFiles:       3,
	}

	assert.Equal(t, int64(1024), update.BytesTransferred)
	assert.Equal(t, int64(2048), update.TotalBytes)
	assert.Equal(t, "test.txt", update.CurrentFile)
	assert.Equal(t, 1, update.CurrentFileNum)
	assert.Equal(t, 3, update.TotalFiles)
}

func TestProgressDone(t *testing.T) {
	t.Run("with error", func(t *testing.T) {
		done := ProgressDone{Error: assert.AnError}
		assert.Error(t, done.Error)
	})

	t.Run("without error", func(t *testing.T) {
		done := ProgressDone{Error: nil}
		assert.NoError(t, done.Error)
	})
}
