package classifiers

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// UniqueLabelOptions contains properties of the unique label classifier.
type UniqueLabelOptions struct {
	Label string
}

// GetLabel returns the unique label for a new task.
func (o *UniqueLabelOptions) GetLabel() string {
	return o.Label
}

// Validate normalizes and validates the classifier options.
func (o *UniqueLabelOptions) Validate() error {
	o.Label = strings.ToLower(strings.TrimSpace(o.Label))

	if len(o.Label) == 0 {
		return fmt.Errorf("empty label")
	}

	return nil
}

// UniqueLabelClassifier is designed to store one-to-one mapping between
// unique labels and task IDs.
type UniqueLabelClassifier struct {
	mu    sync.Mutex
	items map[string]string
}

// NewUniqueLabelClassifier returns a new UniqueLabelClassifier instance.
func NewUniqueLabelClassifier() *UniqueLabelClassifier {
	return &UniqueLabelClassifier{
		items: make(map[string]string),
	}
}

// Assign associates the given task ID (tid) with the unique label specified in opts.
func (c *UniqueLabelClassifier) Assign(ctx context.Context, opts Options, tid string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tid = strings.ToLower(strings.TrimSpace(tid))

	if len(tid) == 0 {
		return fmt.Errorf("unique-label-classifier: %w: empty tid", ErrValidationFailed)
	}

	if opts == nil {
		return fmt.Errorf("unique-label-classifier: %w: empty opts", ErrValidationFailed)
	} else {
		if err := opts.Validate(); err != nil {
			return fmt.Errorf("unique-label-classifier: %w: %w", ErrValidationFailed, err)
		}
	}

	if _, found := c.items[opts.GetLabel()]; found {
		return fmt.Errorf("unique-label-classifier: %w: already exists: %s", ErrAssignmentFailed, opts.GetLabel())
	}

	c.items[opts.GetLabel()] = tid

	return nil
}

// Unassign removes the task entry from the table.
func (c *UniqueLabelClassifier) Unassign(tid string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) == 0 {
		return
	}

	tid = strings.ToLower(strings.TrimSpace(tid))

	for label, v := range c.items {
		if v == tid {
			delete(c.items, label)
		}
	}
}

// Get returns a slice of task IDs assigned to the classifier.
func (c *UniqueLabelClassifier) Get(labels ...string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	tids := make([]string, 0, 1)

	for _, label := range labels {
		if tid, found := c.items[label]; found {
			tids = append(tids, tid)
		}
	}

	return tids
}

// Len returns the number of entries in the classifier table.
func (c *UniqueLabelClassifier) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.items)
}
