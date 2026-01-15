package context

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewContext(t *testing.T) {
	ctx := New()

	assert.NotNil(t, ctx)
	assert.NotNil(t, ctx.Styles)
	assert.NotNil(t, ctx.Tasks)
	assert.Equal(t, 0, ctx.ScreenWidth)
	assert.Equal(t, 0, ctx.ScreenHeight)
}

func TestSetDimensions(t *testing.T) {
	ctx := New()
	ctx.SetDimensions(100, 50, 80, 45)

	assert.Equal(t, 100, ctx.ScreenWidth)
	assert.Equal(t, 50, ctx.ScreenHeight)
	assert.Equal(t, 80, ctx.ContentWidth)
	assert.Equal(t, 45, ctx.ContentHeight)
}

func TestHasActiveTask(t *testing.T) {
	ctx := New()

	// No tasks
	assert.False(t, ctx.HasActiveTask())

	// Add running task
	ctx.Tasks["task1"] = Task{
		ID:          "task1",
		Description: "Loading...",
		State:       TaskRunning,
		StartTime:   time.Now(),
	}
	assert.True(t, ctx.HasActiveTask())

	// Mark as finished
	task := ctx.Tasks["task1"]
	task.State = TaskFinished
	ctx.Tasks["task1"] = task
	assert.False(t, ctx.HasActiveTask())
}

func TestActiveTaskDescription(t *testing.T) {
	ctx := New()

	// No tasks
	assert.Equal(t, "", ctx.ActiveTaskDescription())

	// Add running task
	ctx.Tasks["task1"] = Task{
		ID:          "task1",
		Description: "Loading instances...",
		State:       TaskRunning,
		StartTime:   time.Now(),
	}
	assert.Equal(t, "Loading instances...", ctx.ActiveTaskDescription())

	// Finished task should not return description
	task := ctx.Tasks["task1"]
	task.State = TaskFinished
	ctx.Tasks["task1"] = task
	assert.Equal(t, "", ctx.ActiveTaskDescription())
}

func TestDefaultStyles(t *testing.T) {
	styles := DefaultStyles()

	// Verify color palette
	assert.Equal(t, "#4285F4", string(styles.Colors.Primary))
	assert.Equal(t, "#34A853", string(styles.Colors.Secondary))
	assert.Equal(t, "#FBBC05", string(styles.Colors.Warning))
	assert.Equal(t, "#EA4335", string(styles.Colors.Error))

	// Verify styles are initialized by rendering something
	rendered := styles.Common.Title.Render("test")
	assert.NotEmpty(t, rendered)
}

func TestTaskStates(t *testing.T) {
	// Verify task state constants
	assert.Equal(t, TaskState(0), TaskRunning)
	assert.Equal(t, TaskState(1), TaskFinished)
	assert.Equal(t, TaskState(2), TaskError)
}
