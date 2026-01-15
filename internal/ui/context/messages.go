package context

// TaskStartedMsg is sent when an async task begins
type TaskStartedMsg struct {
	Task Task
}

// TaskFinishedMsg is sent when an async task completes
type TaskFinishedMsg struct {
	TaskID string
	Error  error
}

// TaskClearMsg is sent to remove a completed task from display
type TaskClearMsg struct {
	TaskID string
}
