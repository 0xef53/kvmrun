package classifiers

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// GroupLabelOptions contains properties of the group label classifier.
type GroupLabelOptions struct {
	Label string
}

// GetLabel returns the label associated with the group.
func (o *GroupLabelOptions) GetLabel() string {
	return o.Label
}

// Validate normalizes and validates the classifier options.
func (o *GroupLabelOptions) Validate() error {
	o.Label = strings.ToLower(strings.TrimSpace(o.Label))

	if len(o.Label) == 0 {
		return fmt.Errorf("empty label")
	}

	return nil
}

// GroupLabelClassifier is designed to store tasks with the same group label.
type GroupLabelClassifier struct {
	mu    sync.Mutex
	items map[string]map[string]struct{}
}

// NewGroupLabelClassifier returns a new GroupLabelClassifier instance.
func NewGroupLabelClassifier() *GroupLabelClassifier {
	return &GroupLabelClassifier{
		items: make(map[string]map[string]struct{}),
	}
}

// Assign adds the specified task ID to the table if the task label matches the group label.
func (c *GroupLabelClassifier) Assign(ctx context.Context, opts Options, tid string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tid = strings.ToLower(strings.TrimSpace(tid))

	if len(tid) == 0 {
		return fmt.Errorf("group-label-classifier: %w: empty tid", ErrValidationFailed)
	}

	if opts == nil {
		return fmt.Errorf("group-label-classifier: %w: empty opts", ErrValidationFailed)
	} else {
		if err := opts.Validate(); err != nil {
			return fmt.Errorf("group-label-classifier: %w: %w", ErrValidationFailed, err)
		}
	}

	if group, found := c.items[opts.GetLabel()]; found {
		if _, found := group[tid]; found {
			return fmt.Errorf("group-label-classifier: %w: already exists in group %s: %s", ErrAssignmentFailed, opts.GetLabel(), tid)
		}
		group[tid] = struct{}{}
	} else {
		c.items[opts.GetLabel()] = map[string]struct{}{
			tid: {},
		}
	}

	return nil
}

// Unassign removes the task entry from the table.
func (c *GroupLabelClassifier) Unassign(tid string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) == 0 {
		return
	}

	tid = strings.ToLower(strings.TrimSpace(tid))

	for label, group := range c.items {
		delete(group, tid)

		if len(group) == 0 {
			delete(c.items, label)
		}
	}
}

// Get returns a slice of task IDs assigned to the classifier.
func (c *GroupLabelClassifier) Get(labels ...string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	tids := make([]string, 0, 1)

	for _, label := range labels {
		if group, found := c.items[label]; found {
			for tid := range group {
				tids = append(tids, tid)
			}
		}
	}

	return tids
}

// Len returns the number of entries in the classifier table.
func (c *GroupLabelClassifier) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.items)
}
