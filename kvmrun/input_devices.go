package kvmrun

import (
	"encoding/json"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun/internal/pool"
)

type InputDeviceProperties struct {
	Type string `json:"type"`
}

func (p *InputDeviceProperties) Validate(_ bool) error {
	p.Type = strings.TrimSpace(p.Type)

	return nil
}

type InputDevice struct {
	InputDeviceProperties
}

func NewInputDevice(devtype string) (*InputDevice, error) {
	dev := new(InputDevice)

	dev.Type = devtype

	if err := dev.Validate(true); err != nil {
		return nil, err
	}

	return dev, nil
}

func (d *InputDevice) Copy() *InputDevice {
	return &InputDevice{InputDeviceProperties: d.InputDeviceProperties}
}

type InputDevicePool struct {
	pool.Pool
}

// Get returns a pointer to a InputDevice with type = devtype.
func (p *InputDevicePool) Get(devtype string) (dev *InputDevice) {
	err := p.Pool.GetAs(devtype, &dev)
	if err == nil {
		return dev
	}

	return nil
}

// Exists returns true if a InputDevice with type = devtype is in the pool.
// Otherwise returns false.
func (p *InputDevicePool) Exists(devtype string) bool {
	return p.Pool.Exists(devtype)
}

// Values returns all or specified InputDevices from the pool.
func (p *InputDevicePool) Values(devtypes ...string) []*InputDevice {
	values := make([]*InputDevice, 0, p.Len())

	for _, v := range p.Pool.Values(devtypes...) {
		if d, ok := v.(*InputDevice); ok {
			values = append(values, d)
		}
	}

	return values
}

// Append appends a new InputDevice to the end of the pool.
func (p *InputDevicePool) Append(dev *InputDevice) error {
	return p.Pool.Append(dev.Type, dev, false)
}

// Insert inserts a new InputDevice into the pool at the given position.
func (p *InputDevicePool) Insert(dev *InputDevice, idx int) error {
	return p.Pool.Insert(dev.Type, dev, idx)
}

// Remove removes a InputDevice with type = devtype from the pool.
func (p *InputDevicePool) Remove(devtype string) (err error) {
	return p.Pool.Remove(devtype)
}

func (p *InputDevicePool) UnmarshalJSON(data []byte) (err error) {
	var devices []*InputDevice

	if err := json.Unmarshal(data, &devices); err != nil {
		return err
	}

	for _, dev := range devices {
		if err = p.Pool.Append(dev.Type, dev, false); err != nil {
			return err
		}
	}

	return nil
}
