// Package timer provides countdown and elapsed time components with GCP styling.
package timer

import (
	"time"

	"github.com/charmbracelet/bubbles/timer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GCP color palette
var (
	colorPrimary = lipgloss.Color("#4285F4")
	colorMuted   = lipgloss.Color("#9AA0A6")
	colorWarning = lipgloss.Color("#FBBC05")
	colorError   = lipgloss.Color("#EA4335")
)

// Mode determines the timer behavior
type Mode int

const (
	// Countdown counts down from a duration to zero
	Countdown Mode = iota
	// Stopwatch counts up from zero (uses elapsed time tracking)
	Stopwatch
)

// Model wraps bubbles/timer with GCP styling and additional features
type Model struct {
	timer     timer.Model
	mode      Mode
	duration  time.Duration
	running   bool
	timedOut  bool
	startTime time.Time
	elapsed   time.Duration
	style     lipgloss.Style
	label     string
}

// New creates a new timer model with the specified duration
func New(duration time.Duration, mode Mode) Model {
	t := timer.NewWithInterval(duration, time.Second)

	return Model{
		timer:    t,
		mode:     mode,
		duration: duration,
		running:  false,
		timedOut: false,
		style: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary),
	}
}

// NewCountdown creates a countdown timer
func NewCountdown(duration time.Duration) Model {
	return New(duration, Countdown)
}

// NewStopwatch creates a stopwatch (counts up)
func NewStopwatch() Model {
	// Large duration since stopwatch counts elapsed time
	return New(24*time.Hour, Stopwatch)
}

// WithLabel sets a label to display before the timer
func (m Model) WithLabel(label string) Model {
	m.label = label
	return m
}

// Start begins the timer
func (m *Model) Start() tea.Cmd {
	m.running = true
	m.timedOut = false
	m.startTime = time.Now()
	m.elapsed = 0
	return m.timer.Init()
}

// Stop pauses the timer
func (m *Model) Stop() {
	m.running = false
	m.timer.Stop()
}

// Toggle starts or stops the timer
func (m *Model) Toggle() tea.Cmd {
	if m.running {
		m.Stop()
		return nil
	}
	return m.Start()
}

// Reset resets the timer to initial state
func (m *Model) Reset() {
	m.timer = timer.NewWithInterval(m.duration, time.Second)
	m.running = false
	m.timedOut = false
	m.elapsed = 0
}

// Running returns true if the timer is active
func (m Model) Running() bool {
	return m.running
}

// TimedOut returns true if the countdown reached zero
func (m Model) TimedOut() bool {
	return m.timedOut
}

// Remaining returns the remaining duration (for countdown mode)
func (m Model) Remaining() time.Duration {
	return m.timer.Timeout
}

// Elapsed returns the elapsed duration
func (m Model) Elapsed() time.Duration {
	if m.mode == Stopwatch && m.running {
		return time.Since(m.startTime)
	}
	return m.elapsed
}

// Init initializes the timer (does not start it)
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles timer messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case timer.TickMsg:
		var cmd tea.Cmd
		m.timer, cmd = m.timer.Update(msg)

		if m.mode == Stopwatch {
			m.elapsed = time.Since(m.startTime)
		}

		return m, cmd

	case timer.StartStopMsg:
		var cmd tea.Cmd
		m.timer, cmd = m.timer.Update(msg)
		return m, cmd

	case timer.TimeoutMsg:
		m.timedOut = true
		m.running = false
		return m, nil
	}

	return m, nil
}

// View renders the timer
func (m Model) View() string {
	var timeStr string

	if m.mode == Countdown {
		timeStr = m.formatDuration(m.Remaining())
	} else {
		timeStr = m.formatDuration(m.Elapsed())
	}

	// Apply styling based on state
	style := m.style
	if m.mode == Countdown {
		remaining := m.Remaining()
		switch {
		case m.timedOut:
			style = style.Foreground(colorError)
		case remaining < 10*time.Second:
			style = style.Foreground(colorError)
		case remaining < 30*time.Second:
			style = style.Foreground(colorWarning)
		}
	}

	if !m.running && !m.timedOut {
		style = style.Foreground(colorMuted)
	}

	result := style.Render(timeStr)

	if m.label != "" {
		labelStyle := lipgloss.NewStyle().Foreground(colorMuted)
		result = labelStyle.Render(m.label+": ") + result
	}

	return result
}

// formatDuration formats a duration as HH:MM:SS or MM:SS
func (m Model) formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return formatTimeComponent(hours) + ":" + formatTimeComponent(minutes) + ":" + formatTimeComponent(seconds)
	}
	return formatTimeComponent(minutes) + ":" + formatTimeComponent(seconds)
}

// formatTimeComponent formats a number with leading zero if needed
func formatTimeComponent(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// TimerStartMsg is sent when the timer should start
type TimerStartMsg struct{}

// TimerStopMsg is sent when the timer should stop
type TimerStopMsg struct{}

// TimerResetMsg is sent when the timer should reset
type TimerResetMsg struct{}

// Cmd returns the tick command for the timer
func (m Model) Cmd() tea.Cmd {
	if m.running {
		return m.timer.Init()
	}
	return nil
}
