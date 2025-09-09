package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/0xef53/kvmrun/internal/task/metadata"

	log "github.com/sirupsen/logrus"
)

var (
	ErrTaskNotRunning      = errors.New("process is not running")
	ErrTaskInterrupted     = errors.New("process was interrupted")
	ErrUninterruptibleTask = errors.New("unable to cancel uninterruptible process")
)

type OperationMode uint32

type Task interface {
	Main() error

	BeforeStart(interface{}) error
	OnSuccess() error
	OnFailure(error)

	Wait()
	Cancel() error
	IsRunning() bool

	Err() error
	Ctx() context.Context

	ID() string
	ShortID() string
	CreationTime() time.Time
	ModifiedTime() time.Time

	Targets() map[string]OperationMode

	SetProgress(int)

	Stat() *TaskStat
	Metadata() interface{}
}

// It's an implementation of a generic task
type GenericTask struct {
	sync.Mutex

	id string

	createdAt  time.Time
	modifiedAt time.Time

	Logger *log.Entry

	ctx       context.Context
	cancel    context.CancelFunc
	released  chan struct{}
	completed bool

	progress   int
	progressCh chan<- int

	err error
}

func (t *GenericTask) init(ctx context.Context, id string, progressCh chan<- int) {
	t.Lock()
	defer t.Unlock()

	if cap(progressCh) == 0 {
		panic("unable to work with unbuffered progressCh channel")
	}

	t.id = id

	//ctx = context.WithValue(ctx, "task-id", id)
	//ctx = context.WithValue(ctx, "task-short-id", t.shortID())

	t.ctx, t.cancel = context.WithCancel(ctx)

	t.released = make(chan struct{})

	t.Logger = log.WithField("task-id", t.shortID())

	//if v := ctx.Value("request-id"); v != nil {
	//	t.Logger = t.Logger.WithField("request-id", v.(string))
	//}

	t.createdAt = time.Now()
	t.modifiedAt = t.createdAt

	t.progressCh = progressCh
}

func (t *GenericTask) release(err error) {
	t.Lock()
	defer t.Unlock()

	t.cancel()

	t.cancel = nil
	t.completed = true

	if t.err == nil {
		t.err = err
	} else {
		// In case the task was cancelled manually
		t.err = fmt.Errorf("%w: %w", t.err, err)
	}

	if t.progressCh != nil {
		close(t.progressCh)

		t.progressCh = nil
	}

	close(t.released)
}

func (t *GenericTask) BeforeStart(a interface{}) error {
	return nil
}

func (t *GenericTask) OnSuccess() error {
	return nil
}

func (t *GenericTask) OnFailure(_ error) {
	// return
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

	// This error indicates that the task was manually canceled
	t.err = ErrTaskInterrupted

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

func (t *GenericTask) IsInterrupted() bool {
	return t.Stat().Interrupted
}

func (t *GenericTask) Err() error {
	t.Lock()
	defer t.Unlock()

	return t.err
}

func (t *GenericTask) Stat() *TaskStat {
	t.Lock()
	defer t.Unlock()

	st := TaskStat{
		ID:       t.id,
		ShortID:  t.shortID(),
		Progress: t.progress,
		Metadata: t.Metadata(),
	}

	switch {
	case t.completed:
		if t.err == nil {
			st.State = StateCompleted
		} else {
			st.State = StateFailed
			st.StateDesc = t.err.Error()

			st.Interrupted = errors.Is(t.err, ErrTaskInterrupted)
		}

		// There is no special status to indicate a cancelled task,
		// since there is no way to determine how exactly the cancellation
		// was performed -- via the Cancel() function or in any other way
		// in the main task code (via context or otherwise).
		//
		// Use IsInterrupted() function or flag "Interrupted" as a quick way
		// to find out if the error tree contains an ErrTaskInterrupted error.

	case t.cancel != nil:
		st.State = StateRunning
	}

	return &st
}

func (t *GenericTask) Metadata() interface{} {
	md, ok := metadata.FromContext(t.ctx)
	if ok {
		return md
	}

	return nil
}

func (t *GenericTask) SetProgress(v int) {
	t.Lock()
	defer t.Unlock()

	if v > 0 {
		t.progress = v
	}

	if t.progressCh != nil {
		// Non-blocking send to a buffered channel
		select {
		case t.progressCh <- v:
		default:
		}
	}
}

func (t *GenericTask) Ctx() context.Context {
	t.Lock()
	defer t.Unlock()

	return t.ctx
}

func (t *GenericTask) ID() string {
	t.Lock()
	defer t.Unlock()

	return t.id
}

func (t *GenericTask) shortID() string {
	return strings.Split(t.id, "-")[0]
}

func (t *GenericTask) ShortID() string {
	t.Lock()
	defer t.Unlock()

	return t.shortID()
}

func (t *GenericTask) CreationTime() time.Time {
	t.Lock()
	defer t.Unlock()

	return t.createdAt
}

func (t *GenericTask) ModifiedTime() time.Time {
	t.Lock()
	defer t.Unlock()

	return t.modifiedAt
}

func (t *GenericTask) Targets() map[string]OperationMode {
	return nil
}

type ConcurrentRunningError struct {
	Name    string
	Targets map[string]OperationMode
}

func (e *ConcurrentRunningError) Error() string {
	objects := make([]string, 0, len(e.Targets))

	for obj := range e.Targets {
		objects = append(objects, obj)
	}

	ff := strings.Split(e.Name, ".")

	basename := ff[len(ff)-1]

	return fmt.Sprintf("concurrent process is already running: task = %s, objects = %q", basename, objects)
}

func IsConcurrentRunningError(err error) bool {
	if _, ok := err.(*ConcurrentRunningError); ok {
		return true
	}
	return false
}
