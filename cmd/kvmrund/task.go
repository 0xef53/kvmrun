package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	log "github.com/sirupsen/logrus"
)

var (
	ErrTaskNotRunning = errors.New("process is not running")
)

// A generic task implementation
type task struct {
	mu sync.Mutex

	id     string
	logger *log.Entry

	cancel    context.CancelFunc
	released  chan struct{}
	completed bool

	mon       *QMPPool
	rpcClient *rpcclient.TlsClient

	err error
}

func (t *task) Wait() {
	<-t.released
}

func (t *task) Cancel() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel == nil {
		return ErrTaskNotRunning
	}

	t.cancel()

	return nil
}

func (t *task) InProgress() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.cancel != nil
}

// Err returns the last migration error
func (t *task) Err() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.err
}

type Task interface {
	Start() error
	Wait()
	Cancel() error
	InProgress() bool
	Err() error
}

type TaskPool struct {
	mu    sync.Mutex
	table map[string]Task

	mon       *QMPPool
	rpcClient *rpcclient.TlsClient
}

func NewTaskPool(mon *QMPPool, cli *rpcclient.TlsClient) *TaskPool {
	return &TaskPool{
		table:     make(map[string]Task),
		mon:       mon,
		rpcClient: cli,
	}
}

func (p *TaskPool) Cancel(taskID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if t, found := p.table[taskID]; found {
		// Cancel the existing process ...
		t.Cancel()
		// ... and wait for it to be done
		t.Wait()

		delete(p.table, taskID)
	}
}

func (p *TaskPool) CancelAll(vmname string) {
	for tid := range p.table {
		_, object, owner := parseTaskID(tid)

		if object == vmname || owner == vmname {
			if p.table[tid].InProgress() {
				log.WithFields(log.Fields{"vmname": vmname}).Info("cancelling the task: ", tid)
				p.Cancel(tid)
			}
		}
	}
}

func (p *TaskPool) InProgress(taskID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if t, found := p.table[taskID]; found {
		return t.InProgress()
	}

	return false
}

func (p *TaskPool) Err(taskID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if t, found := p.table[taskID]; found {
		return t.Err()
	}

	return nil
}

func (p *TaskPool) Stat() map[string]bool {
	m := make(map[string]bool)

	for tid := range p.table {
		m[tid] = p.table[tid].InProgress()
	}

	return m
}

func (p *TaskPool) StartMigration(opts *MigrationTaskOpts) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for tid := range p.table {
		tt, object, owner := parseTaskID(tid)

		if object == opts.VMName || owner == opts.VMName {
			if p.table[tid].InProgress() {
				return &TaskAlreadyRunningError{tt, object, owner}
			}
		}
	}

	taskID := "migration:" + opts.VMName

	if _, found := p.table[taskID]; found {
		// This means that it's a struct of a previous migration
		// and now we can remove it
		delete(p.table, taskID)
	}

	t := &MigrationTask{
		task: task{
			id:        taskID,
			logger:    log.WithField("task-id", taskID),
			mon:       p.mon,
			rpcClient: p.rpcClient,
		},
		opts: opts,
	}

	p.table[taskID] = t

	return t.Start()
}

func (p *TaskPool) MigrationStat(taskID string) *rpccommon.MigrationTaskStat {
	p.mu.Lock()
	defer p.mu.Unlock()

	if t, found := p.table[taskID]; found {
		if mt, ok := t.(*MigrationTask); ok {
			return mt.Stat()
		}
	}

	// Otherwise return an empty stat
	return &rpccommon.MigrationTaskStat{
		Status: "none",
		Qemu:   new(rpccommon.StatInfo),
		Disks:  make(map[string]*rpccommon.StatInfo),
	}
}

func (p *TaskPool) StartDiskCopying(opts *DiskCopyingTaskOpts) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	diskname := opts.SrcDisk.BaseName()

	for tid := range p.table {
		tt, object, owner := parseTaskID(tid)

		if object == opts.VMName || (owner == opts.VMName && object == diskname) {
			if p.table[tid].InProgress() {
				return &TaskAlreadyRunningError{tt, object, owner}
			}
		}
	}

	taskID := "disk-copying:" + diskname + "@" + opts.VMName

	if _, found := p.table[taskID]; found {
		// This means that it's a struct of a previous disk copying process
		// and now we can remove it
		delete(p.table, taskID)
	}

	t := &DiskCopyingTask{
		task: task{
			id:        taskID,
			logger:    log.WithField("task-id", taskID),
			mon:       p.mon,
			rpcClient: p.rpcClient,
		},
		opts: opts,
	}

	p.table[taskID] = t

	return t.Start()
}

func (p *TaskPool) DiskCopyingStat(taskID string) *rpccommon.DiskCopyingTaskStat {
	p.mu.Lock()
	defer p.mu.Unlock()

	if t, found := p.table[taskID]; found {
		if dt, ok := t.(*DiskCopyingTask); ok {
			return dt.Stat()
		}
	}

	// Otherwise return an empty stat
	return &rpccommon.DiskCopyingTaskStat{
		Status:  "none",
		QemuJob: new(rpccommon.StatInfo),
	}
}

func (p *TaskPool) StartInit(opts *VMInitTaskOpts) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// No checks are required here,
	// as this process starts when the virtual machine starts

	taskID := "init:" + opts.VMName

	if t, found := p.table[taskID]; found {
		if t.InProgress() {
			return &TaskAlreadyRunningError{"init", opts.VMName, ""}
		}
		// Remove previous copy of the same struct
		delete(p.table, taskID)
	}

	t := &VMInitTask{
		task: task{
			id:        taskID,
			logger:    log.WithField("task-id", taskID),
			mon:       p.mon,
			rpcClient: p.rpcClient,
		},
		opts: opts,
	}

	p.table[taskID] = t

	return t.Start()
}

func parseTaskID(taskID string) (string, string, string) {
	var tt, object, owner string

	fields := strings.Split(taskID, ":")

	tt, object = fields[0], fields[1]

	switch ff := strings.Split(object, "@"); len(ff) {
	case 2:
		object, owner = ff[0], ff[1]
	}

	return tt, object, owner
}

type TaskInterruptedError struct {
	Err error
}

func (e *TaskInterruptedError) Error() string {
	if e.Err != nil {
		return "interrupted error: " + e.Err.Error()
	}
	return "process was interrupted"
}

func IsTaskInterruptedError(err error) bool {
	if _, ok := err.(*TaskInterruptedError); ok {
		return true
	}
	return false
}

type TaskAlreadyRunningError struct {
	Type   string
	Object string
	Owner  string
}

func (e *TaskAlreadyRunningError) Error() string {
	var desc string
	if len(e.Owner) > 0 {
		desc = fmt.Sprintf("another %s process for %s@%s is already running", e.Type, e.Object, e.Owner)
	} else {
		desc = fmt.Sprintf("another %s process for %s is already running", e.Type, e.Object)
	}
	return "unable to start: " + desc
}

func IsTaskAlreadyRunningError(err error) bool {
	if _, ok := err.(*TaskAlreadyRunningError); ok {
		return true
	}
	return false
}
