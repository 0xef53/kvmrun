package classifiers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type LimitedGroupOptions struct{}

func (o *LimitedGroupOptions) GetLabel() string {
	return ""
}

func (o *LimitedGroupOptions) Validate() error {
	return nil
}

type LimitedGroupClassifier struct {
	mu      sync.Mutex
	items   map[string]struct{}
	label   string
	size    uint16
	waitCh  chan struct{}
	timeout time.Duration
}

func NewLimitedGroupClassifier(label string, size uint16, timeout time.Duration) *LimitedGroupClassifier {
	return &LimitedGroupClassifier{
		items:   make(map[string]struct{}),
		waitCh:  make(chan struct{}),
		label:   label,
		size:    size,
		timeout: timeout,
	}
}

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

func (c *LimitedGroupClassifier) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.items)
}
