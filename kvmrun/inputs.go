package kvmrun

import (
	"fmt"
)

type InputDevice struct {
	Type string `json:"type"`
}

type InputPool []InputDevice

// Get returns a pointer to an element with Type == t.
func (p InputPool) Get(t string) *InputDevice {
	for k := range p {
		if p[k].Type == t {
			return &p[k]
		}
	}

	return nil
}

// Exists returns true if an element with Type == t is present in the list.
// Otherwise returns false.
func (p InputPool) Exists(t string) bool {
	for _, d := range p {
		if d.Type == t {
			return true
		}
	}

	return false
}

// Append appends a new element to the end of the list.
func (p *InputPool) Append(d *InputDevice) {
	*p = append(*p, *d)
}

// Remove removes an element with Type == t from the list.
func (p *InputPool) Remove(t string) error {
	for idx, d := range *p {
		if d.Type == t {
			return (*p).RemoveN(idx)
		}
	}

	return fmt.Errorf("device not found: %s", t)
}

// RemoveN removes an element with Index == idx from the list.
func (p *InputPool) RemoveN(idx int) error {
	if !(idx >= 0 && idx <= len(*p)) {
		return fmt.Errorf("invalid device index: %d", idx)
	}

	switch {
	case idx == len(*p):
		*p = (*p)[:idx]
	default:
		*p = append((*p)[:idx], (*p)[idx+1:]...)
	}

	return nil
}
