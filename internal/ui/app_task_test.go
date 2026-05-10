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
