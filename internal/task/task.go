package task

import (
	"context"
	"errors"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	ErrTaskNotRunning      = errors.New("process is not running")
	ErrTaskInterrupted     = errors.New("process was interrupted")
	ErrUninterruptibleTask = errors.New("unable to cancel uninterruptible process")
)

type Task interface {
	Main() error

	BeforeStart(interface{}) error
	OnSuccess() error
	OnFailure() error

	Wait()
	Cancel() error
	IsRunning() bool

	Err() error
	Ctx() context.Context

	GetNS() string
	GetKey() string
	GetCreationTime() time.Time
	GetModifiedTime() time.Time

	SetProgress(int32)

	Stat() *TaskStat
}

// It's an implementation of a generic task
type GenericTask struct {
	sync.Mutex

	key string
	tag string

	createdAt  time.Time
	modifiedAt time.Time

	Logger *log.Entry

	ctx       context.Context
	cancel    context.CancelFunc
	released  chan struct{}
	completed bool

	progress int32

	err error
}

func (t *GenericTask) init(ctx context.Context, key string) {
	t.Lock()
	defer t.Unlock()

	t.ctx, t.cancel = context.WithCancel(ctx)

	t.key = key
	t.released = make(chan struct{})

	t.Logger = log.WithField("task-key", key)

	if v := ctx.Value("tag"); v != nil {
		t.tag = v.(string)
		t.Logger = t.Logger.WithField("task-tag", v.(string))
	}

	t.createdAt = time.Now()
	t.modifiedAt = t.createdAt
}

func (t *GenericTask) release(err error) {
	t.cancel()

	t.cancel = nil
	t.completed = true
	t.err = err

	close(t.released)
}

func (t *GenericTask) BeforeStart(a interface{}) error {
	return nil
}

func (t *GenericTask) OnSuccess() error {
	return nil
}

func (t *GenericTask) OnFailure() error {
	return nil
}

func (t *GenericTask) Wait() {
	<-t.released
}

func (t *GenericTask) Cancel() error {
	t.Lock()
	defer t.Unlock()

	if t.cancel == nil {
		return ErrTaskNotRunning
	}

	t.cancel()

	return nil
}

func (t *GenericTask) IsRunning() bool {
	return t.Stat().State == StateRunning
}

func (t *GenericTask) IsCompleted() bool {
	return t.Stat().State == StateCompleted
}

func (t *GenericTask) IsFailed() bool {
	return t.Stat().State == StateFailed
}

// Err returns the last migration error
func (t *GenericTask) Err() error {
	t.Lock()
	defer t.Unlock()

	return t.err
}

func (t *GenericTask) Stat() *TaskStat {
	t.Lock()
	defer t.Unlock()

	st := TaskStat{
		Key:      t.key,
		Progress: t.progress,
	}

	switch {
	case t.completed:
		if t.err == nil {
			st.State = StateCompleted
		} else {
			st.State = StateFailed
			st.StateDesc = t.err.Error()
		}
	case t.cancel != nil:
		st.State = StateRunning
	}

	return &st
}

func (t *GenericTask) SetProgress(p int32) {
	t.Lock()
	defer t.Unlock()

	if p > 0 {
		t.progress = p
	}
}

func (t *GenericTask) Ctx() context.Context {
	t.Lock()
	defer t.Unlock()

	return t.ctx
}

func (t *GenericTask) GetNS() string {
	return "default"
}

func (t *GenericTask) GetKey() string {
	t.Lock()
	defer t.Unlock()

	return t.key
}

func (t *GenericTask) getTag() string {
	t.Lock()
	defer t.Unlock()

	return t.tag
}

func (t *GenericTask) GetCreationTime() time.Time {
	t.Lock()
	defer t.Unlock()

	return t.createdAt
}

func (t *GenericTask) GetModifiedTime() time.Time {
	t.Lock()
	defer t.Unlock()

	return t.modifiedAt
}

type TaskAlreadyRunningError struct {
	Namespace string
	Key       string
}

func (e *TaskAlreadyRunningError) Error() string {
	return "another process is already running: " + e.Key
}

func IsTaskAlreadyRunningError(err error) bool {
	if _, ok := err.(*TaskAlreadyRunningError); ok {
		return true
	}
	return false
}
