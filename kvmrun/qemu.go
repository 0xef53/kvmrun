package kvmrun

import (
	"fmt"
)

type QemuVersion int

func (v QemuVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v/10000, (v%10000)/100, (v%10000)%100)
}

type QemuChipset int32

const (
	QEMU_CHIPSET_UNKNOWN QemuChipset = iota
	QEMU_CHIPSET_I440FX
	QEMU_CHIPSET_Q35
	QEMU_CHIPSET_MICROVM
)

type QemuMachine struct {
	name string `json:"-"`

	Chipset QemuChipset `json:"type"`
}

func (m *QemuMachine) String() string {
	return m.name
}

type QemuFirmware struct {
	Image string `json:"image,omitempty"`
}
