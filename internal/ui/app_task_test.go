package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/ui/context"
)

func TestUpdateRunningTask_UpdatesDescription(t *testing.T) {
	a := &App{ctx: context.New()}
	a.ctx.Tasks["foo"] = context.Task{
		ID:          "foo",
		Description: "Initial",
		State:       context.TaskRunning,
		StartTime:   time.Now(),
	}
	a.updateRunningTask("foo", "Updated 50%")
	assert.Equal(t, "Updated 50%", a.ctx.Tasks["foo"].Description)
}

func TestUpdateRunningTask_NoOpForFinishedTask(t *testing.T) {
	a := &App{ctx: context.New()}
	a.ctx.Tasks["foo"] = context.Task{
		ID:          "foo",
		Description: "Done",
		State:       context.TaskFinished,
	}
	a.updateRunningTask("foo", "should not apply")
	assert.Equal(t, "Done", a.ctx.Tasks["foo"].Description)
}

func TestUpdateRunningTask_NoOpForUnknownTask(t *testing.T) {
	a := &App{ctx: context.New()}
	// Should not panic.
	a.updateRunningTask("nope", "anything")
	_, exists := a.ctx.Tasks["nope"]
	assert.False(t, exists)
}

// TestTaskClearMsg_DoesNotDeleteRunningTask is the regression for C3: a stale
// TaskClearMsg from a previous (deterministically named) task must not delete
// a re-running task with the same ID.
func TestTaskClearMsg_DoesNotDeleteRunningTask(t *testing.T) {
	a := createTestApp()
	const id = "scan:bucket-x|"

	// Simulate a previous run that finished, then a fresh run starting before
	// the 2-second TaskClearMsg fires.
	a.ctx.Tasks[id] = context.Task{
		ID:          id,
		Description: "fresh scan",
		State:       context.TaskRunning,
		StartTime:   time.Now(),
	}

	// The stale TaskClearMsg arrives.
	_, _ = a.Update(context.TaskClearMsg{TaskID: id})

	task, exists := a.ctx.Tasks[id]
	assert.True(t, exists, "running task must not be deleted by stale TaskClearMsg")
	assert.Equal(t, context.TaskRunning, task.State)
}

// TestTaskClearMsg_DeletesFinishedTask confirms the normal case still works.
func TestTaskClearMsg_DeletesFinishedTask(t *testing.T) {
	a := createTestApp()
	const id = "scan:bucket-y|"
	a.ctx.Tasks[id] = context.Task{
		ID:          id,
		Description: "old scan",
		State:       context.TaskFinished,
		StartTime:   time.Now().Add(-3 * time.Second),
	}

	_, _ = a.Update(context.TaskClearMsg{TaskID: id})

	_, exists := a.ctx.Tasks[id]
	assert.False(t, exists, "finished task should be cleared as before")
}

