package file

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xef53/kvmrun/kvmrun/backend"
)

type Device struct {
	Path string
}

func New(p string) (*Device, error) {
	v, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}

	return &Device{Path: v}, nil
}

func (d *Device) QdevID() string {
	return "blk_" + d.BaseName()
}

func (d *Device) FullPath() string {
	return d.Path
}

func (d *Device) BaseName() string {
	return filepath.Base(d.Path)
}

func (d *Device) Size() (uint64, error) {
	fi, err := os.Lstat(d.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, &os.PathError{Op: "stat", Path: d.Path, Err: os.ErrNotExist}
		} else {
			return 0, err
		}
	}

	if fi.Mode().IsRegular() {
		return uint64(fi.Size()), nil
	}

	return 0, fmt.Errorf("not a regular file: %s", d.Path)
}

func (d *Device) IsLocal() bool {
	return true
}

func (d *Device) IsAvailable() (bool, error) {
	fi, err := os.Lstat(d.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("%w: %s", os.ErrNotExist, d.Path)
		} else {
			return false, err
		}
	}

	if fi.Mode().IsRegular() {
		return true, nil
	}

	return false, fmt.Errorf("not a regular file: %s", d.Path)
}

func (d *Device) Copy() backend.DiskBackend {
	return &Device{
		Path: d.Path,
	}
}
