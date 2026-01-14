package timer

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/timer"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	tm := New(5*time.Minute, Countdown)

	assert.Equal(t, 5*time.Minute, tm.duration)
	assert.Equal(t, Countdown, tm.mode)
	assert.False(t, tm.running)
	assert.False(t, tm.timedOut)
}

func TestNewCountdown(t *testing.T) {
	tm := NewCountdown(10 * time.Second)

	assert.Equal(t, Countdown, tm.mode)
	assert.Equal(t, 10*time.Second, tm.duration)
}

func TestNewStopwatch(t *testing.T) {
	tm := NewStopwatch()

	assert.Equal(t, Stopwatch, tm.mode)
}

func TestWithLabel(t *testing.T) {
	tm := NewCountdown(1 * time.Minute).WithLabel("Time remaining")

	assert.Equal(t, "Time remaining", tm.label)
}

func TestStartAndStop(t *testing.T) {
	tm := NewCountdown(1 * time.Minute)

	assert.False(t, tm.Running())

	tm.Start()
	assert.True(t, tm.Running())

	tm.Stop()
	assert.False(t, tm.Running())
}

func TestToggle(t *testing.T) {
	tm := NewCountdown(1 * time.Minute)

	assert.False(t, tm.Running())

	tm.Toggle()
	assert.True(t, tm.Running())

	tm.Toggle()
	assert.False(t, tm.Running())
}

func TestReset(t *testing.T) {
	tm := NewCountdown(1 * time.Minute)
	tm.Start()
	tm.timedOut = true

	tm.Reset()

	assert.False(t, tm.Running())
	assert.False(t, tm.TimedOut())
	assert.Equal(t, time.Duration(0), tm.elapsed)
}

func TestRemaining(t *testing.T) {
	tm := NewCountdown(5 * time.Minute)

	// Initial remaining should equal duration
	remaining := tm.Remaining()
	assert.Equal(t, 5*time.Minute, remaining)
}

func TestElapsed(t *testing.T) {
	tm := NewStopwatch()

	// Not started, elapsed should be 0
	assert.Equal(t, time.Duration(0), tm.Elapsed())
}

func TestInit(t *testing.T) {
	tm := NewCountdown(1 * time.Minute)
	cmd := tm.Init()

	// Init should return nil (timer doesn't start automatically)
	assert.Nil(t, cmd)
}

func TestUpdateTickMsg(t *testing.T) {
	tm := NewCountdown(1 * time.Minute)
	tm.Start()

	// Simulate tick message
	updated, _ := tm.Update(timer.TickMsg{})
	assert.NotNil(t, updated)
}

func TestUpdateTimeoutMsg(t *testing.T) {
	tm := NewCountdown(1 * time.Second)
	tm.Start()

	// Simulate timeout
	updated, _ := tm.Update(timer.TimeoutMsg{})

	assert.True(t, updated.TimedOut())
	assert.False(t, updated.Running())
}

func TestViewCountdown(t *testing.T) {
	tm := NewCountdown(5*time.Minute + 30*time.Second)

	view := tm.View()

	assert.Contains(t, view, "05:30")
}

func TestViewWithLabel(t *testing.T) {
	tm := NewCountdown(1 * time.Minute).WithLabel("Timeout")

	view := tm.View()

	assert.Contains(t, view, "Timeout")
}

func TestViewStopwatch(t *testing.T) {
	tm := NewStopwatch()

	view := tm.View()

	// Should show 00:00 initially
	assert.Contains(t, view, "00:00")
}

func TestFormatDuration(t *testing.T) {
	tm := NewCountdown(1 * time.Hour)

	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "00:00"},
		{30 * time.Second, "00:30"},
		{5 * time.Minute, "05:00"},
		{5*time.Minute + 30*time.Second, "05:30"},
		{1 * time.Hour, "01:00:00"},
		{1*time.Hour + 30*time.Minute + 45*time.Second, "01:30:45"},
		{-1 * time.Second, "00:00"}, // Negative should show 00:00
	}

	for _, tt := range tests {
		result := tm.formatDuration(tt.duration)
		assert.Equal(t, tt.expected, result, "duration: %v", tt.duration)
	}
}

func TestFormatTimeComponent(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "00"},
		{5, "05"},
		{9, "09"},
		{10, "10"},
		{59, "59"},
	}

	for _, tt := range tests {
		result := formatTimeComponent(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestCmd(t *testing.T) {
	tm := NewCountdown(1 * time.Minute)

	// Not running, should return nil
	assert.Nil(t, tm.Cmd())

	tm.Start()
	// Running, should return command
	assert.NotNil(t, tm.Cmd())
}

func TestModeConstants(t *testing.T) {
	assert.Equal(t, Mode(0), Countdown)
	assert.Equal(t, Mode(1), Stopwatch)
}
