package kvmrun

import (
	"fmt"
	"os"
	"strings"
)

// ExtKernel represents the guest kernel configuration structure.
type ExtKernelProperties struct {
	Image   string `json:"image,omitempty"`
	Cmdline string `json:"cmdline,omitempty"`
	Initrd  string `json:"initrd,omitempty"`
	Modiso  string `json:"modiso,omitempty"`
}

func (p *ExtKernelProperties) Validate(strict bool) error {
	p.Image = strings.TrimSpace(p.Image)
	p.Cmdline = strings.TrimSpace(p.Cmdline)
	p.Initrd = strings.TrimSpace(p.Initrd)
	p.Modiso = strings.TrimSpace(p.Modiso)

	if len(p.Image) == 0 {
		return fmt.Errorf("empty kernel image path")
	}

	if strict {
		for _, fname := range []string{p.Image, p.Initrd, p.Modiso} {
			if len(fname) > 0 {
				if _, err := os.Stat(fname); err != nil {
					if os.IsNotExist(err) {
						return err
					}
					return fmt.Errorf("failed to check %s: %w", fname, err)
				}
			}
		}
	}

	return nil
}

// ExtKernel represents the guest kernel configuration structure.
type ExtKernel struct {
	ExtKernelProperties
}

func NewExtKernel(image, cmdline, initrd, modiso string) (*ExtKernel, error) {
	k := new(ExtKernel)

	k.Image = image
	k.Cmdline = cmdline
	k.Initrd = initrd
	k.Modiso = modiso

	if err := k.Validate(false); err != nil {
		return nil, err
	}

	return k, nil
}

func (k *ExtKernel) Copy() *ExtKernel {
	return &ExtKernel{ExtKernelProperties: k.ExtKernelProperties}
}

func (k *ExtKernel) SetImage(value string) error {
	k.Image = strings.TrimSpace(value)

	return nil
}

func (k *ExtKernel) SetCmdline(value string) error {
	k.Cmdline = strings.TrimSpace(value)

	return nil
}

func (k *ExtKernel) SetInitrd(value string) error {
	k.Initrd = strings.TrimSpace(value)

	return nil
}

func (k *ExtKernel) SetModiso(value string) error {
	k.Modiso = strings.TrimSpace(value)

	return nil
}

func (k *ExtKernel) Reset() error {
	k.Image = ""
	k.Cmdline = ""
	k.Initrd = ""
	k.Modiso = ""

	return nil
}
