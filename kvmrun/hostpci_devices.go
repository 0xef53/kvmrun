package kvmrun

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/internal/pci"
	"github.com/0xef53/kvmrun/kvmrun/internal/pool"
)

type HostDeviceProperties struct {
	PCIAddr       string `json:"addr"` // in format [domain:]bus:device.function
	PrimaryGPU    bool   `json:"primary_gpu"`
	Multifunction bool   `json:"multifunction"`
}

func (p *HostDeviceProperties) Validate(strict bool) error {
	if v, err := pci.AddressFromHex(strings.TrimSpace(p.PCIAddr)); err == nil {
		p.PCIAddr = v.String() // normalized addr
	} else {
		return err
	}

	if strict {
		if _, err := pci.LookupDevice(p.PCIAddr); err != nil {
			return err
		}
	}

	return nil
}

type HostDevice struct {
	HostDeviceProperties

	BackendAddr *pci.Address `json:"-"`
}

func NewHostDevice(hexaddr string) (*HostDevice, error) {
	dev := new(HostDevice)

	dev.PCIAddr = hexaddr

	if err := dev.Validate(false); err != nil {
		return nil, err
	}

	if addr, err := pci.AddressFromHex(dev.PCIAddr); err == nil {
		dev.BackendAddr = addr
	} else {
		return nil, err
	}

	return dev, nil
}

func (d *HostDevice) Copy() *HostDevice {
	v := HostDevice{HostDeviceProperties: d.HostDeviceProperties}

	if d.BackendAddr != nil {
		v.BackendAddr = &pci.Address{
			Domain:   d.BackendAddr.Domain,
			Bus:      d.BackendAddr.Bus,
			Device:   d.BackendAddr.Device,
			Function: d.BackendAddr.Function,
		}
	}

	return &v
}

type HostDevicePool struct {
	pool.Pool
}

// Get returns a pointer to a HostPCI-device with addr = hexaddr.
func (p *HostDevicePool) Get(hexaddr string) (dev *HostDevice) {
	if addr, err := pci.AddressFromHex(strings.TrimSpace(hexaddr)); err == nil {
		if err := p.Pool.GetAs(addr.String(), &dev); err == nil {
			return dev
		}
	}

	return nil
}

// Exists returns true if a HostPCI-device with addr = hexaddr is in the pool.
// Otherwise returns false.
func (p *HostDevicePool) Exists(hexaddr string) bool {
	if addr, err := pci.AddressFromHex(strings.TrimSpace(hexaddr)); err == nil {
		return p.Pool.Exists(addr.String())
	}

	return false
}

// Values returns all or specified HostPCI-devices from the pool.
func (p *HostDevicePool) Values(hexaddrs ...string) []*HostDevice {
	validAddrs := make([]string, 0, len(hexaddrs))

	for _, s := range hexaddrs {
		if addr, err := pci.AddressFromHex(strings.TrimSpace(s)); err == nil {
			validAddrs = append(validAddrs, addr.String())
		}
	}

	values := make([]*HostDevice, 0, p.Len())

	for _, v := range p.Pool.Values(validAddrs...) {
		if d, ok := v.(*HostDevice); ok {
			values = append(values, d)
		}
	}

	return values
}

// Append appends a new HostPCI-device to the end of the pool.
func (p *HostDevicePool) Append(dev *HostDevice) error {
	if dev.BackendAddr == nil {
		return fmt.Errorf("invalid host-pci addr")
	}

	return p.Pool.Append(dev.BackendAddr.String(), dev, false)
}

// Insert inserts a new HostPCI-device into the pool at the given position.
func (p *HostDevicePool) Insert(dev *HostDevice, idx int) error {
	if dev.BackendAddr == nil {
		return fmt.Errorf("invalid host-pci addr")
	}

	return p.Pool.Insert(dev.BackendAddr.String(), dev, idx)
}

// Remove removes a HostPCI-device with addr = hexaddr from the pool.
func (p *HostDevicePool) Remove(hexaddr string) (err error) {
	addr, err := pci.AddressFromHex(strings.TrimSpace(hexaddr))
	if err != nil {
		return err
	}

	return p.Pool.Remove(addr.String())
}

func (p *HostDevicePool) UnmarshalJSON(data []byte) (err error) {
	var devices []*HostDevice

	if err := json.Unmarshal(data, &devices); err != nil {
		return err
	}

	for _, dev := range devices {
		if addr, err := pci.AddressFromHex(dev.PCIAddr); err == nil {
			dev.PCIAddr = addr.String()
			dev.BackendAddr = addr
		}

		if err = p.Pool.Append(dev.BackendAddr.String(), dev, false); err != nil {
			return err
		}
	}

	return nil
}
