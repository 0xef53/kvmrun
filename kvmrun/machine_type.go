package kvmrun

import (
	"fmt"
	"strings"
)

type QemuChipset int32

const (
	QEMU_CHIPSET_UNKNOWN QemuChipset = iota
	QEMU_CHIPSET_I440FX
	QEMU_CHIPSET_Q35
	QEMU_CHIPSET_MICROVM
)

type MachineType struct {
	name string `json:"-"`

	Chipset QemuChipset `json:"type"`
}

func ParseMachineType(typename string) (*MachineType, error) {
	var chipset string

	ff := strings.Split(typename, "-")

	switch len(ff) {
	case 3:
		chipset = ff[1]
	case 1:
		chipset = ff[0]
	}

	switch chipset {
	case "", "pc", "i440fx":
		return &MachineType{name: typename, Chipset: QEMU_CHIPSET_I440FX}, nil
	case "q35":
		return &MachineType{name: typename, Chipset: QEMU_CHIPSET_Q35}, nil
	case "microvm":
		return &MachineType{name: typename, Chipset: QEMU_CHIPSET_MICROVM}, nil
	}

	return nil, fmt.Errorf("unsupported machine type: %s", typename)
}

func (m *MachineType) String() string {
	return m.name
}
