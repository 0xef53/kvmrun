package kvmrun

import (
	"fmt"
)

type VirtioChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Addr string `json:"addr,omitempty"`
}

func (c *VirtioChannel) QdevID() string {
	return fmt.Sprintf("cdev_%s", c.ID)
}

func (c *VirtioChannel) CharDevName() string {
	return fmt.Sprintf("c_%s", c.ID)
}

type Channels []VirtioChannel

// Get returns a pointer to an element with ID == id.
func (cc Channels) Get(id string) *VirtioChannel {
	for k, _ := range cc {
		if cc[k].ID == id {
			return &cc[k]
		}
	}
	return nil
}

// Exists returns true if an element with ID == id is present in the list.
// Otherwise returns false.
func (cc Channels) Exists(id string) bool {
	for _, c := range cc {
		if c.ID == id {
			return true
		}
	}
	return false
}

// NameExists returns true if an element with Name == name is present in the list.
// Otherwise returns false.
func (cc Channels) NameExists(name string) bool {
	for _, c := range cc {
		if c.Name == name {
			return true
		}
	}
	return false
}

// Append appends a new element to the end of the list.
func (cc *Channels) Append(c *VirtioChannel) {
	*cc = append(*cc, *c)
}

// Insert inserts a new element into the list at a given position.
func (cc *Channels) Insert(c *VirtioChannel, idx int) error {
	if idx < 0 {
		return fmt.Errorf("Invalid channel index: %d", idx)
	}
	*cc = append(*cc, VirtioChannel{})
	copy((*cc)[idx+1:], (*cc)[idx:])
	(*cc)[idx] = *c
	return nil
}

// Remove removes an element with ID == id from the list.
func (cc *Channels) Remove(id string) error {
	for idx, c := range *cc {
		if c.ID == id {
			return (*cc).RemoveN(idx)
		}
	}
	return fmt.Errorf("Channel not found: %s", id)
}

// RemoveN removes an element with Index == idx from the list.
func (cc *Channels) RemoveN(idx int) error {
	if !(idx >= 0 && idx <= len(*cc)) {
		return fmt.Errorf("Invalid channel index: %d", idx)
	}
	switch {
	case idx == len(*cc):
		*cc = (*cc)[:idx]
	default:
		*cc = append((*cc)[:idx], (*cc)[idx+1:]...)
	}
	return nil
}
