package kvmrun

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun/backend"
	"github.com/0xef53/kvmrun/kvmrun/backend/block"
	"github.com/0xef53/kvmrun/kvmrun/backend/file"
)

type FirmwareProperties struct {
	Image string `json:"image,omitempty"`
	Flash string `json:"flash,omitempty"`
}

func (p *FirmwareProperties) Validate(strict bool) error {
	p.Image = strings.TrimSpace(p.Image)
	p.Flash = strings.TrimSpace(p.Flash)

	if len(p.Image) == 0 {
		return fmt.Errorf("empty firmware image path")
	}

	if strict {
		if _, err := os.Stat(p.Image); err != nil {
			if os.IsNotExist(err) {
				return err
			}
			return fmt.Errorf("failed to check image file: %w", err)
		}

		if len(p.Flash) > 0 {
			if _, err := os.Stat(p.Flash); err != nil {
				if os.IsNotExist(err) {
					return err
				}
				return fmt.Errorf("failed to check flash file: %w", err)
			}
		}
	}

	return nil
}

type FirmwareFlashBackend struct {
	backend.DiskBackend
}

func (b *FirmwareFlashBackend) QdevID() string {
	return "fwflash"
}

func (b *FirmwareFlashBackend) BaseName() string {
	return "fwflash"
}

func NewFirmwareFlashBackend(p string) (backend.DiskBackend, error) {
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

	return &FirmwareFlashBackend{b}, nil
}

type Firmware struct {
	FirmwareProperties

	flashDisk *Disk `json:"-"`
}

func NewFirmware(image, flash string) (*Firmware, error) {
	fw := new(Firmware)

	fw.Image = image
	fw.Flash = flash

	if err := fw.Validate(false); err != nil {
		return nil, err
	}

	if len(fw.Flash) > 0 {
		if d, err := newFirmwareFlashDisk(fw.Flash); err == nil {
			fw.flashDisk = d
		} else {
			return nil, err
		}
	}

	return fw, nil
}

func (fw *Firmware) Copy() *Firmware {
	v := Firmware{FirmwareProperties: fw.FirmwareProperties}

	if fw.flashDisk != nil {
		v.flashDisk = fw.flashDisk.Copy()
	}

	return &v
}

func (fw *Firmware) SetImage(image string) error {
	image = strings.TrimSpace(image)

	if len(image) == 0 {
		return fmt.Errorf("empty firmware image path")
	}

	fw.Image = image

	return nil
}

func (fw *Firmware) SetFlash(flash string) error {
	flash = strings.TrimSpace(flash)

	if len(flash) == 0 {
		return fmt.Errorf("empty firmware pflash path")
	}

	fw.Flash = flash

	if d, err := newFirmwareFlashDisk(flash); err == nil {
		fw.flashDisk = d
	} else {
		return err
	}

	return nil
}

func (fw *Firmware) UnmarshalJSON(data []byte) (err error) {
	opts := FirmwareProperties{}

	if err := json.Unmarshal(data, &opts); err != nil {
		return err
	}

	opts.Flash = strings.TrimSpace(opts.Flash)

	fw.Image = opts.Image
	fw.Flash = opts.Flash

	if len(opts.Flash) > 0 {

		if d, err := newFirmwareFlashDisk(opts.Flash); err == nil {
			fw.flashDisk = d
		}
	}

	return nil
}

func newFirmwareFlashDisk(p string) (*Disk, error) {
	d := new(Disk)

	d.Path = p

	d.driver = DriverType_UNKNOWN
	d.DiskProperties.Driver = "pflash"

	if be, err := NewFirmwareFlashBackend(p); err == nil {
		d.Backend = be
	} else {
		return nil, err
	}

	return d, nil
}
