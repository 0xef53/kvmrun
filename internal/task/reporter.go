package task

import (
	"context"
)

// Reporter defines a set of methods for handling notifications about changes
// in a task state -- status, progress, detail statistics.
type Reporter interface {
	// Send is called every time the task status changes.
	Send(context.Context, *TaskStat)

	// SendProgress is called when the task starts and receives progress updates.
	// The progress value (in percent) should be set during execution using SetProgress().
	SendProgress(ctx context.Context, taskID string, progressCh <-chan int)
}
