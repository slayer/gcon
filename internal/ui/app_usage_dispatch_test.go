package ui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/gcp/usage"
	uictx "github.com/slayer/gcon/internal/ui/context"
)

// fakeMonitoring satisfies usage.MonitoringFetcher with no-op behavior.
// Only used to construct a Scanner; tests below never trigger a fetch.
type fakeMonitoring struct{}

func (fakeMonitoring) FetchBucketUsage(_ context.Context, _ string) (bytes, count int64, asOf time.Time, err error) {
	return 0, 0, time.Time{}, nil
}

// fakeStorage satisfies usage.StorageLister with no-op behavior.
type fakeStorage struct{}

func (fakeStorage) ListAllObjects(_ context.Context, _, _ string) ([]gcp.StorageObject, error) {
	return nil, nil
}

// TestHandleUsageReady_NoMessageReemission verifies that handleUsageReady does
// not re-emit usage.ReadyMsg via the returned tea.Cmd batch. Re-emission
// would re-enter App.Update for the same case → unbounded recursion + repeated
// finishTask calls (C1).
func TestHandleUsageReady_NoMessageReemission(t *testing.T) {
	a := &App{ctx: uictx.New()}
	a.usageScanner = usage.New(fakeStorage{}, fakeMonitoring{})

	// Pre-register a running task so finishTask has work to do.
	jobID := "scan:bucket-a|"
	a.registerRunningTask(jobID, "scan in progress")
	require.Equal(t, uictx.TaskRunning, a.ctx.Tasks[jobID].State)

	// Drive a single ReadyMsg through the handler.
	cmd := a.handleUsageReady(usage.ReadyMsg{
		JobID: jobID,
		Usage: usage.BucketUsage{Bucket: "bucket-a"},
	})

	// Task transitioned to Finished after one call.
	assert.Equal(t, uictx.TaskFinished, a.ctx.Tasks[jobID].State,
		"finishTask should have been invoked exactly once")

	// Inspect every message produced by the returned cmd batch. None of them
	// may be a usage.ReadyMsg (which would re-enter the Update case and recurse).
	require.NotNil(t, cmd)
	msgs := drainCmd(cmd)
	for _, m := range msgs {
		_, isReady := m.(usage.ReadyMsg)
		assert.False(t, isReady,
			"handler must NOT re-emit usage.ReadyMsg; got %T", m)
	}
}

// TestHandleUsageProgress_NoMessageReemission is the equivalent regression for
// ProgressMsg. The handler must call view.Update directly rather than wrapping
// the same message into a Cmd that will re-enter App.Update.
func TestHandleUsageProgress_NoMessageReemission(t *testing.T) {
	a := &App{ctx: uictx.New()}
	a.usageScanner = usage.New(fakeStorage{}, fakeMonitoring{})

	jobID := "scan:bucket-a|"
	a.registerRunningTask(jobID, "scan in progress")

	cmd := a.handleUsageProgress(usage.ProgressMsg{
		JobID:  jobID,
		Bucket: "bucket-a",
	})

	require.NotNil(t, cmd)
	msgs := drainCmd(cmd)
	for _, m := range msgs {
		_, isProgress := m.(usage.ProgressMsg)
		assert.False(t, isProgress,
			"handler must NOT re-emit usage.ProgressMsg; got %T", m)
	}
}

// TestHandleUsageReady_NilScanner_NoPanic verifies the early bail-out when the
// scanner has been dropped (e.g., after a project switch). An in-flight
// ProgressMsg/ReadyMsg arriving after clearAllViews() must not panic (C2).
func TestHandleUsageReady_NilScanner_NoPanic(t *testing.T) {
	a := &App{ctx: uictx.New()}
	a.usageScanner = nil

	assert.NotPanics(t, func() {
		_ = a.handleUsageReady(usage.ReadyMsg{JobID: "scan:b|"})
	})
	assert.NotPanics(t, func() {
		_ = a.handleUsageProgress(usage.ProgressMsg{JobID: "scan:b|", Bucket: "b"})
	})
}

// drainCmd executes a tea.Cmd in a worker goroutine and collects messages
// produced within a short window. Bubble Tea wraps batches as tea.BatchMsg;
// we recurse into them so nested cmds are visible. Cmds that block longer
// than the per-cmd timeout (e.g. tea.Tick scheduling a TaskClearMsg 2s out)
// are abandoned — the test only cares about messages that re-enter App.Update
// synchronously, which would happen immediately.
func drainCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	out := []tea.Msg{}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if msg == nil {
			return out
		}
		switch v := msg.(type) {
		case tea.BatchMsg:
			for _, sub := range v {
				out = append(out, drainCmd(sub)...)
			}
		default:
			out = append(out, msg)
		}
	case <-time.After(50 * time.Millisecond):
		// Cmd is doing real work (e.g. tea.Tick); ignore — it can't be a
		// synchronous re-emission that would cause the bug we're guarding against.
	}
	return out
}
