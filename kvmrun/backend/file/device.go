package file

import (
	"fmt"
	"os"
	"path/filepath"
)

type Device struct {
	Path string
}

func New(p string) (*Device, error) {
	return &Device{Path: p}, nil
}

func (d *Device) QdevID() string {
	return "blk_" + d.BaseName()
}

func (d *Device) BaseName() string {
	return filepath.Base(d.Path)
}

func (d *Device) Size() (uint64, error) {
	fi, err := os.Lstat(d.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, &os.PathError{"stat", d.Path, os.ErrNotExist}
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
			return false, &os.PathError{"stat", d.Path, os.ErrNotExist}
		} else {
			return false, err
		}
	}

	if fi.Mode().IsRegular() {
		return true, nil
	}

	return false, fmt.Errorf("not a regular file: %s", d.Path)
}
