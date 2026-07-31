package kvmrun

import (
	"encoding/json"
	"strings"
)

type QemuVgaType uint16

const (
	QEMU_VGA_CIRRUS QemuVgaType = iota
	QEMU_VGA_STD
)

func (t QemuVgaType) String() string {
	switch t {
	case QEMU_VGA_STD:
		return "std"
	}

	return "cirrus"
}

func QemuVgaTypeValue(s string) QemuVgaType {
	switch strings.ToLower(s) {
	case "std":
		return QEMU_VGA_STD
	}

	return QEMU_VGA_CIRRUS
}

func DefaultQemuVgaType() QemuVgaType {
	return QEMU_VGA_CIRRUS
}

type VgaDeviceProperties struct {
	Type string `json:"type"`
}

func (p *VgaDeviceProperties) Validate(_ bool) error {
	p.Type = strings.TrimSpace(p.Type)

	if len(p.Type) == 0 {
		p.Type = DefaultQemuVgaType().String()
	} else {
		p.Type = QemuVgaTypeValue(p.Type).String()
	}

	return nil
}

type VgaDevice struct {
	VgaDeviceProperties

	vgaType QemuVgaType
}

func NewVgaDevice(t string) (*VgaDevice, error) {
	dev := new(VgaDevice)

	dev.Type = t

	if err := dev.Validate(true); err != nil {
		return nil, err
	}

	dev.vgaType = QemuVgaTypeValue(dev.Type)

	return dev, nil
}

func (d *VgaDevice) Copy() *VgaDevice {
	v := VgaDevice{VgaDeviceProperties: d.VgaDeviceProperties}

	v.vgaType = d.vgaType

	return &v
}

func (d *VgaDevice) GetType() QemuVgaType {
	return d.vgaType
}

func (d *VgaDevice) SetType(t string) error {
	d.Type = t

	if err := d.Validate(false); err != nil {
		return err
	}

	d.vgaType = QemuVgaTypeValue(d.Type)

	return nil
}

func (d *VgaDevice) UnmarshalJSON(data []byte) (err error) {
	opts := VgaDeviceProperties{}

	if err := json.Unmarshal(data, &opts); err != nil {
		return err
	}

	d.Type = opts.Type

	d.vgaType = QemuVgaTypeValue(opts.Type)

	return nil
}
