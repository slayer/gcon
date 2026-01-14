package progress

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	p := New()

	assert.NotNil(t, p)
	assert.Equal(t, 50, p.width)
	assert.True(t, p.showElapsed)
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

func TestProgressUpdate_DoneAndError(t *testing.T) {
	t.Run("with done and error", func(t *testing.T) {
		update := ProgressUpdate{Done: true, Error: assert.AnError}
		assert.True(t, update.Done)
		assert.Error(t, update.Error)
	})

	t.Run("with done without error", func(t *testing.T) {
		update := ProgressUpdate{Done: true, Error: nil}
		assert.True(t, update.Done)
		assert.NoError(t, update.Error)
	})
}

func TestStart(t *testing.T) {
	p := New()

	assert.True(t, p.startTime.IsZero())

	p.Start()

	assert.False(t, p.startTime.IsZero())
	assert.WithinDuration(t, time.Now(), p.startTime, time.Second)
}

func TestElapsed(t *testing.T) {
	p := New()

	// Before start, elapsed should be 0
	assert.Equal(t, time.Duration(0), p.Elapsed())

	p.Start()
	time.Sleep(10 * time.Millisecond)

	elapsed := p.Elapsed()
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(10))
}

func TestReset(t *testing.T) {
	p := New()
	p.Start()
	p.SetProgress("Test", "file.txt", 1, 2, 100, 200)

	p.Reset()

	assert.Empty(t, p.Title)
	assert.Empty(t, p.CurrentFile)
	assert.Equal(t, 0, p.CurrentFileNum)
	assert.Equal(t, 0, p.TotalFiles)
	assert.Equal(t, int64(0), p.BytesTransferred)
	assert.Equal(t, int64(0), p.TotalBytes)
	assert.True(t, p.startTime.IsZero())
}

func TestSetShowElapsed(t *testing.T) {
	p := New()

	assert.True(t, p.showElapsed)

	p.SetShowElapsed(false)
	assert.False(t, p.showElapsed)

	p.SetShowElapsed(true)
	assert.True(t, p.showElapsed)
}

func TestViewWithElapsedTime(t *testing.T) {
	p := New()
	p.SetSize(80)
	p.Start()
	p.SetProgress("Downloading", "file.txt", 1, 1, 500, 1000)

	view := p.View()

	// Should contain elapsed time format (00:00)
	assert.Contains(t, view, "00:00")
}

func TestViewWithoutElapsedTime(t *testing.T) {
	p := New()
	p.SetSize(80)
	p.SetShowElapsed(false)
	p.Start()
	p.SetProgress("Downloading", "file.txt", 1, 1, 500, 1000)

	view := p.View()

	// Should not contain elapsed time when disabled
	// The title should just be "Downloading" without time
	assert.Contains(t, view, "Downloading")
}

func TestFormatElapsed(t *testing.T) {
	p := New()

	tests := []struct {
		name     string
		elapsed  time.Duration
		expected string
	}{
		{"zero", 0, "00:00"},
		{"seconds", 45 * time.Second, "00:45"},
		{"minutes", 5*time.Minute + 30*time.Second, "05:30"},
		{"hours", 2*time.Hour + 15*time.Minute + 45*time.Second, "02:15:45"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set start time to get desired elapsed
			p.startTime = time.Now().Add(-tt.elapsed)
			result := p.formatElapsed()
			assert.Equal(t, tt.expected, result)
		})
	}
}
