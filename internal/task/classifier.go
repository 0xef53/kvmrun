package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/0xef53/kvmrun/internal/task/classifiers"

	"github.com/google/uuid"
)

var (
	ErrRegistrationFailed = errors.New("cannot register")
	ErrAssignmentFailed   = errors.New("cannot assign")
)

// TaskClassifier defines the interface for managing task classification.
type TaskClassifier interface {
	Assign(context.Context, classifiers.Options, string) error
	Unassign(string)
	Get(...string) []string
	Len() int
}

// TaskClassifierDefinition represents the configuration of a classifier for a specific task.
type TaskClassifierDefinition struct {
	Name string
	Opts classifiers.Options
}

// Validate checks the [TaskClassifierDefinition] for correctness.
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

// rootClassifier manages a set of [TaskClassifier] instances.
type rootClassifier struct {
	mu      sync.Mutex
	table   map[string]TaskClassifier
	aliases map[string]string
}

// newRootClassifier returns a new [rootClassifier] instance.
func newRootClassifier() *rootClassifier {
	return &rootClassifier{
		table:   make(map[string]TaskClassifier),
		aliases: make(map[string]string),
	}
}

// Register registers a [TaskClassifier] instance under one or more names.
// If multiple names are specified, the first one will be the primary one,
// and the rest will be aliases.
// If no names are provided, a default name is generated based on the classifier's type.
//
// Returns the list of registered names or an error if any name already exists
// or the classifier is nil.
func (r *rootClassifier) Register(c TaskClassifier, names ...string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if c == nil {
		return nil, fmt.Errorf("%w: empty classifier interface", ErrRegistrationFailed)
	}

	if len(names) == 0 {
		ff := strings.Split(strings.TrimLeft(fmt.Sprintf("%T", c), "*"), ".")

		names = []string{
			fmt.Sprintf("%s-%s", ff[len(ff)-1], strings.Split(uuid.New().String(), "-")[0]),
		}
	}

	for _, n := range names {
		if _, found := r.aliases[n]; found {
			return nil, fmt.Errorf("%w: name already exists: %s", ErrRegistrationFailed, n)
		}
	}

	for _, n := range names {
		r.aliases[n] = names[0]
	}

	r.table[names[0]] = c

	return names, nil
}

// Deregister removes the [TaskClassifier] and all associated aliases from the list.
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

// Assign associates a task identified by tid with a classifier defined by def.
func (r *rootClassifier) Assign(ctx context.Context, def *TaskClassifierDefinition, tid string) error {
	if def == nil {
		return fmt.Errorf("%w: empty classifier definition", ErrAssignmentFailed)
	}

	if err := def.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrAssignmentFailed, err)
	}

	r.mu.Lock()

	if c, found := r.table[def.Name]; found {
		r.mu.Unlock()

		return c.Assign(ctx, def.Opts, tid)
	}

	r.mu.Unlock()

	return fmt.Errorf("%w: classifier not found: %s", ErrAssignmentFailed, def.Name)
}

// Unassign removes the association for the task identified by tid from all registered classifiers.
func (r *rootClassifier) Unassign(tid string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, c := range r.table {
		c.Unassign(tid)
	}
}

// Get returns a slice of task IDs from all registered classifiers
// that match the provided labels.
func (r *rootClassifier) Get(labels ...string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	tids := make([]string, 0, 1)

	for _, c := range r.table {
		tids = append(tids, c.Get(labels...)...)
	}

	return tids
}
