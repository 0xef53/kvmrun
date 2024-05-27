package kvmrun

import (
	"fmt"

	"github.com/0xef53/kvmrun/internal/pci"
)

type HostPCI struct {
	Addr          string `json:"addr"` // in format [domain:]bus:device.function
	PrimaryGPU    bool   `json:"primary_gpu"`
	Multifunction bool   `json:"multifunction"`

	BackendAddr *pci.Address `json:"-"`
}

func NewHostPCI(hexaddr string) (*HostPCI, error) {
	addr, err := pci.AddressFromHex(hexaddr)
	if err != nil {
		return nil, err
	}

	return &HostPCI{
		Addr:        addr.String(), // normalized addr
		BackendAddr: addr,
	}, nil
}

type HostPCIPool []HostPCI

// Get returns a pointer to an element with requested address.
func (p HostPCIPool) Get(hexaddr string) *HostPCI {
	for idx := range p {
		if string(p[idx].Addr) == hexaddr {
			return &p[idx]
		}
	}

	return nil
}

// Exists returns true if an element with requested address is present in the list.
// Otherwise returns false.
func (p HostPCIPool) Exists(hexaddr string) bool {
	for _, d := range p {
		if string(d.Addr) == hexaddr {
			return true
		}
	}

	return false
}

// Append appends a new element to the end of the list.
func (p *HostPCIPool) Append(d *HostPCI) {
	*p = append(*p, *d)
}

// Remove removes an element with requested address from the list.
func (p *HostPCIPool) Remove(hexaddr string) error {
	for idx, d := range *p {
		if string(d.Addr) == hexaddr {
			return (*p).RemoveN(idx)
		}
	}

	return fmt.Errorf("PCI device not found: %s", hexaddr)
}

// RemoveN removes an element with Index == idx from the list.
func (p *HostPCIPool) RemoveN(idx int) error {
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
