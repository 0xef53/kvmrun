package task

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/0xef53/kvmrun/internal/task/classifiers"

	"github.com/google/uuid"
)

type TaskClassifier interface {
	Assign(context.Context, classifiers.Options, string) error
	Unassign(string)
	Get(...string) []string
	Len() int
}

type TaskClassifierDefinition struct {
	Name string
	Opts classifiers.Options
}

func (o *TaskClassifierDefinition) Validate() error {
	o.Name = strings.TrimSpace(o.Name)

	if len(o.Name) == 0 {
		return fmt.Errorf("empty classifier name")
	}

	if o.Opts == nil {
		return fmt.Errorf("empty classifier options")
	}

	return nil
}

type rootClassifier struct {
	mu      sync.Mutex
	table   map[string]TaskClassifier
	aliases map[string]string
}

func newRootClassifier() *rootClassifier {
	return &rootClassifier{
		table:   make(map[string]TaskClassifier),
		aliases: make(map[string]string),
	}
}

func (r *rootClassifier) Register(c TaskClassifier, names ...string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if c == nil {
		return nil, fmt.Errorf("cannot register: empty classifier interface")
	}

	if len(names) == 0 {
		ff := strings.Split(strings.TrimLeft(fmt.Sprintf("%T", c), "*"), ".")

		names = []string{
			fmt.Sprintf("%s-%s", ff[len(ff)-1], strings.Split(uuid.New().String(), "-")[0]),
		}
	}

	for _, n := range names {
		if _, found := r.aliases[n]; found {
			return nil, fmt.Errorf("cannot register: name already exists: %s", n)
		}
	}

	for _, n := range names {
		r.aliases[n] = names[0]
	}

	r.table[names[0]] = c

	return names, nil
}

func (r *rootClassifier) Deregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var mainName string

	for alias, v := range r.aliases {
		if alias == name {
			mainName = v

			delete(r.aliases, alias)

			break
		}
	}

	for alias, v := range r.aliases {
		if v == mainName {
			delete(r.aliases, alias)
		}
	}

	delete(r.table, mainName)

	return nil
}

func (r *rootClassifier) Assign(ctx context.Context, def *TaskClassifierDefinition, tid string) error {
	if err := def.Validate(); err != nil {
		return fmt.Errorf("cannot assign: %w", err)
	}

	r.mu.Lock()

	if c, found := r.table[def.Name]; found {
		r.mu.Unlock()

		return c.Assign(ctx, def.Opts, tid)
	}

	r.mu.Unlock()

	return fmt.Errorf("cannot assign: classifier not found: %s", def.Name)
}

func (r *rootClassifier) Unassign(tid string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, c := range r.table {
		c.Unassign(tid)
	}
}

func (r *rootClassifier) Get(labels ...string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	tids := make([]string, 0, 1)

	for _, c := range r.table {
		tids = append(tids, c.Get(labels...)...)
	}

	return tids
}
