package task

import (
	log "github.com/sirupsen/logrus"
)

// FuncTask is a task implementation that executes a given function.
// It embeds [GenericTask] to reuse common task fields and behavior.
type FuncTask struct {
	*GenericTask

	targets map[string]OperationMode
	fn      func(*log.Entry) error
}

func (t *FuncTask) Targets() map[string]OperationMode { return t.targets }

// Main runs the given task function and returns the error
// produced by the function, if any.
func (t *FuncTask) Main() error {
	return t.fn(t.Logger)
}
