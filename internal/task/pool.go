package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	ErrPoolClosed = errors.New("pool is closed")
)

type Pool struct {
	mu    sync.Mutex
	table map[string]Task
	depth int

	wg       sync.WaitGroup
	isClosed bool
}

func NewPool(depth int) *Pool {
	return &Pool{
		table: make(map[string]Task),
		depth: depth,
	}
}

func (p *Pool) StartTask(ctx context.Context, t Task, resp interface{}) (string, error) {
	if p.isClosed {
		return "", ErrPoolClosed
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
		init(context.Context, string)
		release(error)
		getTag() string
	})
	if !ok {
		return "", fmt.Errorf("invalid embedded interface")
	}

	if len(strings.SplitN(t.GetKey(), ":", p.depth)) != p.depth {
		return "", fmt.Errorf("invalid task key format: %s", t.GetKey())
	}

	newKey := t.GetNS() + ":" + t.GetKey()

	conflict := func(s1, s2 string) bool {
		// Split by fields without the namespace
		f1 := strings.SplitN(s1, ":", 1+p.depth)[1:]
		f2 := strings.SplitN(s2, ":", 1+p.depth)[1:]

		if strings.Join(f1, ":") == strings.Join(f2, ":") {
			// Tasks are identical
			return true
		}

		for i := range f1 {
			if len(f1[i]) == 0 || len(f2[i]) == 0 {
				return true
			}
			if f1[i] != f2[i] {
				break
			}
		}

		return false
	}

	p.mu.Lock()

	// Get all running tasks and check if a new task conflicts with them
	for key := range p.table {
		if p.table[key].IsRunning() {
			if conflict(key, newKey) {
				p.mu.Unlock()

				return "", &TaskAlreadyRunningError{p.table[key].GetNS(), key}
			}
		}
	}

	if _, found := p.table[newKey]; found {
		// Now this means that it's a struct of a previous task
		// and now we can remove it
		delete(p.table, newKey)
	}

	// Initialize ...
	eti.init(ctx, newKey)

	p.table[newKey] = t

	logger := log.WithFields(log.Fields{"task-key": newKey, "task-tag": eti.getTag()})

	p.mu.Unlock()

	// ... and run the pre-start hook
	if err := t.BeforeStart(resp); err != nil {
		logger.Errorf("Pre-start function failed: %s", err)

		eti.release(err)

		p.mu.Lock()
		delete(p.table, newKey)
		p.mu.Unlock()

		return "", err
	}

	success = true

	// Main background process
	go func() {
		var err error

		defer func() {
			eti.release(err)

			p.wg.Done()

			go func() {
				time.Sleep(30 * time.Second)

				p.mu.Lock()
				defer p.mu.Unlock()

				if t, found := p.table[newKey]; found && !t.IsRunning() {
					delete(p.table, newKey)
				}
			}()
		}()

		err = t.Main()

		if err == nil {
			logger.Info("Successfully completed")

			err = t.OnSuccess()
		} else {
			logger.Errorf("Fatal error: %s", err)

			t.OnFailure()
		}
	}()

	return newKey, nil
}

func (p *Pool) Stat(key string) *TaskStat {
	p.mu.Lock()
	defer p.mu.Unlock()

	if t, found := p.table[key]; found {
		return t.Stat()
	}

	return nil
}

func (p *Pool) Err(key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if t, found := p.table[key]; found {
		return t.Err()
	}

	return nil
}

func (p *Pool) Cancel(key string) {
	t := func() Task {
		p.mu.Lock()
		defer p.mu.Unlock()
		if t, found := p.table[key]; found {
			return t
		}
		return nil
	}()

	if t != nil {
		t.Cancel()
	}
}

func (p *Pool) Wait(key string) {
	t := func() Task {
		p.mu.Lock()
		defer p.mu.Unlock()
		if t, found := p.table[key]; found {
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

	for key := range p.table {
		tasks = append(tasks, key)
	}

	return tasks
}

func (p *Pool) WaitAndClosePool() {
	p.wg.Wait()

	p.isClosed = true
}

func (p *Pool) RunFunc(ctx context.Context, key string, fn func(*log.Entry) error) (string, error) {
	task := FuncTask{new(GenericTask), key, fn}

	key, err := p.StartTask(ctx, &task, nil)
	if err != nil {
		return "", err
	}

	task.Wait()

	return key, task.Err()
}
