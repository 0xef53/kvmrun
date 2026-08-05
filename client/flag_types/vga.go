package flag_types

import (
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun"
)

type VgaDeviceType struct {
	vgaType kvmrun.QemuVgaType
}

func DefaultVgaDeviceType() *VgaDeviceType {
	return &VgaDeviceType{vgaType: kvmrun.QEMU_VGA_CIRRUS}
}

func (t *VgaDeviceType) Set(value string) error {
	value = strings.TrimSpace(value)

	vgaType := kvmrun.QemuVgaTypeValue(value)

	if vgaType.String() != value {
		return fmt.Errorf("unknown VGA type: %s", value)
	}

	t.vgaType = vgaType

	return nil
}

func (t VgaDeviceType) String() string {
	return t.vgaType.String()
}

func (t VgaDeviceType) Get() interface{} {
	return t.vgaType
}
