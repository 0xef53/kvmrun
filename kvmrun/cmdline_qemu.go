package kvmrun

import (
	"fmt"
)

type CommandLineFeatures struct {
	NoReboot     bool
	IncomingHost string
	VNCHost      string
}

type commandLineBuilder interface {
	gen() ([]string, error)
}

type qemuCommandLine struct {
	vmconf   Instance
	features *CommandLineFeatures
}

func (b qemuCommandLine) IncomingHost() string {
	if b.features != nil && len(b.features.IncomingHost) > 0 {
		return b.features.IncomingHost
	}
	return "0.0.0.0"
}

func (b qemuCommandLine) VNCHost() string {
	if b.features != nil && len(b.features.VNCHost) > 0 {
		return b.features.VNCHost
	}
	return "127.0.0.2"
}

func GetCommandLine(vmi Instance, features *CommandLineFeatures) ([]string, error) {
	if features == nil {
		features = new(CommandLineFeatures)
	}

	switch vmi.GetMachineType().Chipset {
	case QEMU_CHIPSET_I440FX:
		return (&qemuCommandLine_i440fx{&qemuCommandLine{vmi, features}}).gen()
	}

	return nil, fmt.Errorf("unsupported machine type: %s", vmi.GetMachineType())
}
