package task

import (
	log "github.com/sirupsen/logrus"
)

type FuncTask struct {
	*GenericTask

	targets map[string]OperationMode
	fn      func(*log.Entry) error
}

func (t *FuncTask) Targets() map[string]OperationMode { return t.targets }

func (t *FuncTask) Main() error {
	return t.fn(t.Logger)
}
