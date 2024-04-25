package kvmrun

import (
	"strings"

	"github.com/0xef53/kvmrun/kvmrun/backend"
	"github.com/0xef53/kvmrun/kvmrun/backend/block"
	"github.com/0xef53/kvmrun/kvmrun/backend/file"
)

type QemuFirmware struct {
	Image string `json:"image,omitempty"`
	Flash string `json:"flash,omitempty"`

	flashDisk *Disk `json:"-"`
}

type FirmwareBackend struct {
	backend.DiskBackend
}

func (b *FirmwareBackend) QdevID() string {
	return "fwflash"
}

func (b *FirmwareBackend) BaseName() string {
	return "fwflash"
}

func NewFirmwareBackend(p string) (backend.DiskBackend, error) {
	var b backend.DiskBackend
	var err error

	switch {
	case strings.HasPrefix(p, "/dev/"):
		b, err = block.New(p)
	default:
		b, err = file.New(p)
	}

	if err != nil {
		return nil, err
	}

	return &FirmwareBackend{b}, nil
}
