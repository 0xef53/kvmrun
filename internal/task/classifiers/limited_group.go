package classifiers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// LimitedGroupOptions contains properties of the limited group classifier.
type LimitedGroupOptions struct{}

// GetLabel returns an empty string because this classifier does not use labels.
func (o *LimitedGroupOptions) GetLabel() string {
	return ""
}

// Validate normalizes and validates the classifier options.
func (o *LimitedGroupOptions) Validate() error {
	return nil
}

// LimitedGroupClassifier is designed to store a fixed-size group
// of concurrently running tasks.
// When the maximum group size is reached, attempts to add new tasks block
// until an existing task completes and frees up space in the group.
//
// The waiting time for adding a new task is limited by a configurable timeout.
type LimitedGroupClassifier struct {
	mu      sync.Mutex
	items   map[string]struct{}
	label   string
	size    uint16
	waitCh  chan struct{}
	timeout time.Duration
}

// NewLimitedGroupClassifier returns a new LimitedGroupClassifier instance
// configured with the specified group size and a timeout duration that limits
// how long a task addition will wait when all slots in the group are occupied.
func NewLimitedGroupClassifier(label string, size uint16, timeout time.Duration) *LimitedGroupClassifier {
	return &LimitedGroupClassifier{
		items:   make(map[string]struct{}),
		waitCh:  make(chan struct{}),
		label:   label,
		size:    size,
		timeout: timeout,
	}
}

// Assign attempts to add the given task ID (tid) to the limited group.
// If the group has reached its maximum size, it blocks until a slot became available.
func (c *LimitedGroupClassifier) Assign(ctx context.Context, _ Options, tid string) error {
	tid = strings.ToLower(strings.TrimSpace(tid))

	if len(tid) == 0 {
		return fmt.Errorf("limited-group-classifier: %w: empty tid", ErrValidationFailed)
	}

	c.mu.Lock()

	if c.items == nil {
		c.items = make(map[string]struct{})
	}

	if _, found := c.items[tid]; found {
		return fmt.Errorf("limited-group-classifier: %w: already exists: %s", ErrAssignmentFailed, tid)
	}

	c.mu.Unlock()

	if len(c.items) == int(c.size) {
		select {
		case <-c.waitCh:
		case <-ctx.Done():
			return fmt.Errorf("limited-group-classifier: %w", ctx.Err())
		case <-time.After(c.timeout):
			return fmt.Errorf("limited-group-classifier: %w", ErrAssignmentTimeout)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[tid] = struct{}{}

	return nil
}

// Unassign removes the task entry from the table.
func (c *LimitedGroupClassifier) Unassign(tid string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) == 0 {
		return
	}

	tid = strings.ToLower(strings.TrimSpace(tid))

	if _, found := c.items[tid]; found {
		delete(c.items, tid)

		if len(c.items) == (int(c.size) - 1) {
			select {
			case c.waitCh <- struct{}{}:
			default:
			}
		}
	}
}

// Get returns a slice of task IDs assigned to the classifier.
func (c *LimitedGroupClassifier) Get(labels ...string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, label := range labels {
		if label == c.label {
			tids := make([]string, 0, len(c.items))

			for tid := range c.items {
				tids = append(tids, tid)
			}

			return tids
		}
	}

	return nil
}

// Len returns the number of entries in the classifier table.
func (c *LimitedGroupClassifier) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.items)
}
