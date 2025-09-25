package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/0xef53/kvmrun/internal/task/metadata"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

var (
	ErrTaskNotRunning      = errors.New("process is not running")
	ErrTaskInterrupted     = errors.New("process was interrupted")
	ErrUninterruptibleTask = errors.New("unable to cancel uninterruptible process")
)

// OperationMode defines bit flags representing different operation modes
// or actions that can be applied to a target or component within a task.
//
// Example:
//
//	modeChangePropertyName task.OperationMode = 1 << (16 - 1 - iota)
//	modeChangePropertyDiskName
//	modeChangePropertyDiskSize
//	modeChangePropertyNetName
//	modeChangePropertyNetLink
//	modePowerUp
//	modePowerDown
//	modePowerCycle
//
//	modeAny                = ^task.OperationMode(0)
//	modeChangePropertyDisk = modeChangePropertyDiskName | modeChangePropertyDiskSize
//	modeChangePropertyNet  = modeChangePropertyNetName | modeChangePropertyNetLink
//	modeChangeProperties   = modeChangePropertyDisk | modeChangePropertyNet
//	modePowerManagement    = modePowerUp | modePowerDown | modePowerCycle
type OperationMode uint32

// Task defines the interface for asynchronous task.
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

type taskInfoKey struct{}

type taskInfo struct {
	TaskID      string    `json:"task_id"`
	TaskShortID string    `json:"task_short_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// InfoFromContext returns the task info from ctx.
func InfoFromContext(ctx context.Context) (*taskInfo, bool) {
	if a := ctx.Value(taskInfoKey{}); a != nil {
		if v, ok := a.(*taskInfo); ok {
			return v, true
		}
	}

	return nil, false
}

// GenericTask is a thread-safe implementation of a generic task.
//
// This struct is designed to be embedded in custom task types to leverage
// common fields and methods.
//
// Example:
//
//	type VirtMachineMigrationTask struct {
//		*task.GenericTask
//
//		targets map[string]task.OperationMode
//
//		// Arguments
//		vmname    string
//		dstServer string
//	}
//
//	func NewVirtMachineMigrationTask(vmname, dstServer string) *VirtMachineMigrationTask {
//		return &VirtMachineMigrationTask{
//			GenericTask: new(task.GenericTask),
//
//			targets:   server.BlockAnyOperations(vmname),
//			vmname:    vmname,
//			dstServer: dstServer,
//		}
//	}
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

// Function initializes the task instance with the provided context, task ID
// and progress channel.
//
// It panics if the given progress channel is unbuffered.
func (t *GenericTask) init(ctx context.Context, id string, progressCh chan<- int) {
	t.Lock()
	defer t.Unlock()

	if cap(progressCh) == 0 {
		panic("unable to work with unbuffered progressCh channel")
	}

	t.id = id

	t.createdAt = time.Now()
	t.modifiedAt = t.createdAt

	ctx = context.WithValue(ctx, taskInfoKey{}, &taskInfo{
		TaskID:      id,
		TaskShortID: t.shortID(),
		CreatedAt:   t.createdAt,
	})

	t.ctx, t.cancel = context.WithCancel(ctx)

	t.released = make(chan struct{})

	t.Logger = log.WithField("task-id", t.shortID())

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

// BeforeStart is a hook called before the task starts.
// It should be overridden to perform any setup or validation.
//
// By default, it does nothing and returns nil.
//
// Example:
//
//	func init() {
//		Pool = task.NewPool()
//	}
//
//	func StartIncomingMigrationProcess(ctx context.Context, vmname string) (*Requisites, error) {
//		requisites := Requisites{}
//
//		t := IncomingMigrationTask{
//			GenericTask: new(task.GenericTask),
//			vmname: vmname,
//		}
//
//		_, err := Pool.TaskStart(ctx, &t, &requisites)
//		if err != nil {
//			return nil, fmt.Errorf("cannot start incoming instance: %w", err)
//		}
//
//		return &requisites, nil
//	}
//
//	type IncomingMigrationTask struct {
//		*task.GenericTask
//
//		vmname string
//	}
//
//	func (t *IncomingMigrationTask) BeforeStart(resp interface{}) error {
//		// some code here ...
//
//		if v, ok := resp.(*Requisites); ok && resp != nil {
//			v.IncomingAddr = incomingAddr
//			v.IncomingPort = incomingPort
//		} else {
//			return fmt.Errorf("invalid type of resp interface")
//		}
//
//		return nil
//	}
func (t *GenericTask) BeforeStart(_ interface{}) error {
	return nil
}

// OnSuccess is a hook called after successful task completion.
// It can be overridden to perform any post-processing.
//
// By default, it does nothing and returns nil.
func (t *GenericTask) OnSuccess() error {
	return nil
}

// OnFailure is a hook called after task failure with the encountered error.
// It can be overridden to handle failure scenarios. For example, to call
// the clean-up code.
//
// By default, it does nothing.
func (t *GenericTask) OnFailure(_ error) {
	// return
}

// Wait blocks until the task is released, i.e., completed or cancelled or failed.
func (t *GenericTask) Wait() {
	<-t.released
}

// Cancel attempts to cancel the running task by invoking its cancel function.
// Returns ErrTaskNotRunning if the task is not currently running.
//
// Sets the task error to ErrTaskInterrupted to indicate manual cancellation.
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

// IsRunning returns true if the task is currently running.
func (t *GenericTask) IsRunning() bool {
	return t.Stat().State == StateRunning
}

// IsCompleted returns true if the task has completed successfully.
func (t *GenericTask) IsCompleted() bool {
	return t.Stat().State == StateCompleted
}

// IsFailed returns true if the task has completed with a failure.
func (t *GenericTask) IsFailed() bool {
	return t.Stat().State == StateFailed
}

// IsInterrupted returns true if the task was interrupted (manually cancelled).
func (t *GenericTask) IsInterrupted() bool {
	return t.Stat().Interrupted
}

// Err returns the error associated with the task, if any.
func (t *GenericTask) Err() error {
	t.Lock()
	defer t.Unlock()

	return t.err
}

// Stat returns the current status of the task, including ID, progress,
// state, and any error information.
//
// It is safe for concurrent use.
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

// Metadata returns user-defined data extracted from the task's context.
// Returns nil if no metadata is found.
func (t *GenericTask) Metadata() interface{} {
	md, ok := metadata.FromContext(t.ctx)
	if ok {
		return md
	}

	return nil
}

// SetProgress updates the progress value and sends it to the progress channel (if available).
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

// Ctx returns the context associated with the task.
func (t *GenericTask) Ctx() context.Context {
	t.Lock()
	defer t.Unlock()

	return t.ctx
}

// ID returns the full task ID.
func (t *GenericTask) ID() string {
	t.Lock()
	defer t.Unlock()

	return t.id
}

func (t *GenericTask) shortID() string {
	if err := uuid.Validate(t.id); err == nil {
		return strings.Split(t.id, "-")[0]
	}

	return t.id
}

// ShortID returns a short version of the task ID.
// If the task ID is a valid UUID, it returns the prefix before the first hyphen.
// Otherwise, it returns the full task ID as is.
func (t *GenericTask) ShortID() string {
	t.Lock()
	defer t.Unlock()

	return t.shortID()
}

// CreationTime returns the time when the task was created.
func (t *GenericTask) CreationTime() time.Time {
	t.Lock()
	defer t.Unlock()

	return t.createdAt
}

// ModifiedTime returns the time of the last task state or progress update.
// This value is refreshed whenever the task status or progress changes.
//
// It is safe for concurrent use.
func (t *GenericTask) ModifiedTime() time.Time {
	t.Lock()
	defer t.Unlock()

	return t.modifiedAt
}

// Targets returns a map of target names to their blocking modes for the task.
//
// By default, it returns nil and should be overridden if any locks are needed
// during the execution.
func (t *GenericTask) Targets() map[string]OperationMode {
	return nil
}

// ConcurrentRunningError represents an error indicating that there is
// an existing task in the pool whose targets partially or completely
// match the new one.
type ConcurrentRunningError struct {
	Name    string
	Targets map[string]OperationMode
}

// Error implements the error interface for ConcurrentRunningError.
// It returns a formatted error message including the task name and a list of target objects.
func (e *ConcurrentRunningError) Error() string {
	objects := make([]string, 0, len(e.Targets))

	for obj := range e.Targets {
		objects = append(objects, obj)
	}

	ff := strings.Split(e.Name, ".")

	basename := ff[len(ff)-1]

	return fmt.Sprintf("concurrent process is already running: task = %s, objects = %q", basename, objects)
}

// IsConcurrentRunningError checks if the given error is of type ConcurrentRunningError.
func IsConcurrentRunningError(err error) bool {
	if _, ok := err.(*ConcurrentRunningError); ok {
		return true
	}

	return false
}
