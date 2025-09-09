package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

var (
	ErrPoolClosed = errors.New("pool is closed")
)

type TaskOption interface{}

type Pool struct {
	mu    sync.Mutex
	table map[string]Task

	reporter Reporter

	classifier *rootClassifier

	wg       sync.WaitGroup
	isClosed bool
}

func NewPool() *Pool {
	return &Pool{
		table:      make(map[string]Task),
		classifier: newRootClassifier(),
	}
}

func (p *Pool) SetReporter(r Reporter) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.reporter = r
}

func (p *Pool) sendReport(ctx context.Context, t Task) {
	if p.reporter == nil {
		return
	}

	p.reporter.Send(ctx, t.Stat())
}

func (p *Pool) RegisterClassifier(c TaskClassifier, names ...string) ([]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.classifier.Register(c, names...)
}

func (p *Pool) StartTask(ctx context.Context, t Task, resp interface{}, opts ...TaskOption) (string, error) {
	err := func() error {
		if p.isClosed {
			return ErrPoolClosed
		}

		var success bool

		p.wg.Add(1)
		defer func() {
			if !success {
				p.wg.Done()
			}
		}()

		// The low level embedded task interface
		eti, ok := t.(interface {
			init(context.Context, string, chan<- int)
			release(error)
		})
		if !ok {
			return fmt.Errorf("invalid embedded interface")
		}

		// New task ID
		tid := uuid.New().String()

		// Parse task options
		for _, opt := range opts {
			switch o := opt.(type) {
			case *TaskClassifierDefinition:
				if err := p.classifier.Assign(ctx, o, tid); err != nil {
					return err
				}
			}
		}
		defer func() {
			if !success {
				p.classifier.Unassign(tid)
			}
		}()

		// Verify if the context was closed in the previous step
		if err := ctx.Err(); err != nil {
			return err
		}

		p.mu.Lock()

		// Get all running tasks and check if a new task conflicts with them
		for tid := range p.table {
			if targets := p.table[tid].Targets(); p.table[tid].IsRunning() && len(targets) > 0 {
				for object, newActions := range t.Targets() {
					if _, ok := targets[object]; ok && targets[object]&newActions != 0 {
						p.mu.Unlock()

						return &ConcurrentRunningError{fmt.Sprintf("%T", t), targets}
					}
				}
			}
		}

		// Will be closed in the task release() function
		progressCh := make(chan int, 8)

		// Initialize ...
		eti.init(ctx, tid, progressCh)

		p.table[t.ID()] = t

		logger := log.WithFields(log.Fields{"task-id": t.ShortID()})

		p.mu.Unlock()

		p.sendReport(ctx, t)

		// Progress reporter
		if p.reporter != nil {
			go p.reporter.SendProgress(ctx, t.ID(), progressCh)
		}

		// ... and run the pre-start hook
		if err := t.BeforeStart(resp); err != nil {
			logger.Errorf("Pre-start function failed: %s", err)

			eti.release(err)

			p.mu.Lock()
			delete(p.table, t.ID())
			p.mu.Unlock()

			return err
		}

		success = true

		// Main background process
		go func() {
			var err error

			defer func() {
				eti.release(err)

				p.classifier.Unassign(t.ID())

				p.wg.Done()

				p.sendReport(ctx, t)

				go func() {
					time.Sleep(30 * time.Second)

					p.mu.Lock()
					defer p.mu.Unlock()

					if t, found := p.table[t.ID()]; found && !t.IsRunning() {
						delete(p.table, t.ID())
					}
				}()
			}()

			err = t.Main()

			if err == nil {
				logger.Info("Successfully completed")

				err = t.OnSuccess()
			} else {
				logger.Errorf("Fatal error: %s", err)

				t.OnFailure(err)
			}
		}()

		return nil
	}()

	if err != nil {
		return "", err
	}

	return t.ID(), nil
}

func (p *Pool) Stat(tid string) *TaskStat {
	p.mu.Lock()
	defer p.mu.Unlock()

	if t, found := p.table[tid]; found {
		return t.Stat()
	}

	return nil
}

func (p *Pool) StatByLabel(labels ...string) []*TaskStat {
	p.mu.Lock()
	defer p.mu.Unlock()

	tids := p.classifier.Get(labels...)

	stats := make([]*TaskStat, 0, len(tids))

	for _, tid := range tids {
		if t, found := p.table[tid]; found {
			stats = append(stats, t.Stat())
		}
	}

	return stats
}

func (p *Pool) Metadata(tid string) interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	if t, found := p.table[tid]; found {
		return t.Metadata()
	}

	return nil
}

func (p *Pool) MetadataByLabel(labels ...string) []interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	tids := p.classifier.Get(labels...)

	data := make([]interface{}, 0, len(tids))

	for _, tid := range tids {
		if t, found := p.table[tid]; found {
			data = append(data, t.Metadata())
		}
	}

	return data
}

func (p *Pool) Err(tid string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if t, found := p.table[tid]; found {
		return t.Err()
	}

	return nil
}

func (p *Pool) Cancel(tid string) {
	t := func() Task {
		p.mu.Lock()
		defer p.mu.Unlock()

		if t, found := p.table[tid]; found {
			return t
		}

		return nil
	}()

	if t != nil {
		t.Cancel()
	}
}

func (p *Pool) CancelByLabel(labels ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	tids := p.classifier.Get(labels...)

	for _, tid := range tids {
		if t, found := p.table[tid]; found {
			t.Cancel()
		}
	}
}

func (p *Pool) Wait(tid string) {
	t := func() Task {
		p.mu.Lock()
		defer p.mu.Unlock()
		if t, found := p.table[tid]; found {
			return t
		}
		return nil
	}()

	if t != nil {
		t.Wait()
	}
}

func (p *Pool) List() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	tasks := make([]string, 0, len(p.table))

	for tid := range p.table {
		tasks = append(tasks, tid)
	}

	return tasks
}

func (p *Pool) WaitAndClosePool() {
	p.wg.Wait()

	p.isClosed = true
}

func (p *Pool) RunFunc(ctx context.Context, tgt map[string]OperationMode, wait bool, opts []TaskOption, fn func(*log.Entry) error) (string, error) {
	task := FuncTask{new(GenericTask), tgt, fn}

	tid, err := p.StartTask(ctx, &task, nil, opts...)
	if err != nil {
		return "", err
	}

	if wait {
		task.Wait()
	}

	return tid, task.Err()
}

func GetShortID(tid string) (string, error) {
	uuid, err := uuid.Parse(tid)
	if err != nil {
		return "", fmt.Errorf("broken UUID: %w", err)
	}

	return strings.Split(uuid.String(), "-")[0], nil
}
