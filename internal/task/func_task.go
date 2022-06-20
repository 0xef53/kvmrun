package task

import (
	log "github.com/sirupsen/logrus"
)

type FuncTask struct {
	*GenericTask

	key string
	fn  func(*log.Entry) error
}

func (t *FuncTask) GetKey() string { return t.key }

func (t *FuncTask) Main() error {
	return t.fn(t.Logger)
}
