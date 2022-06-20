package kvmrun

import (
	"fmt"
)

type Proxy struct {
	Path    string            `json:"path"`
	Command string            `json:"command"`
	Envs    map[string]string `json:"envs"`
}

type ProxyPool []Proxy

// Get returns a pointer to an element with Path == fullpath.
func (p ProxyPool) Get(fullpath string) *Proxy {
	for k, _ := range p {
		if p[k].Path == fullpath {
			return &p[k]
		}
	}

	return nil
}

// Exists returns true if an element with Path == fullpath is present in the list.
// Otherwise returns false.
func (p ProxyPool) Exists(fullpath string) bool {
	for _, d := range p {
		if d.Path == fullpath {
			return true
		}
	}

	return false
}

// Append appends a new element to the end of the list.
func (p *ProxyPool) Append(proxy *Proxy) {
	*p = append(*p, *proxy)
}

// Insert inserts a new element into the list at a given position.
func (p *ProxyPool) Insert(proxy *Proxy, idx int) error {
	if idx < 0 {
		return fmt.Errorf("invalid index: %d", idx)
	}

	*p = append(*p, Proxy{})
	copy((*p)[idx+1:], (*p)[idx:])
	(*p)[idx] = *proxy

	return nil
}

// Remove removes an element with Path == fullpath from the list.
func (p *ProxyPool) Remove(fullpath string) error {
	for idx, d := range *p {
		if d.Path == fullpath {
			return (*p).RemoveN(idx)
		}
	}

	return fmt.Errorf("device not found: %s", p)
}

// RemoveN removes an element with Index == idx from the list.
func (p *ProxyPool) RemoveN(idx int) error {
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
