package block

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
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
	return GetSize64(d.Path)
}

func (d *Device) IsLocal() bool {
	return true
}

func (d *Device) IsAvailable() (bool, error) {
	var st unix.Stat_t

	switch err := unix.Stat(d.Path, &st); {
	case err == nil:
		if (st.Mode & unix.S_IFMT) != unix.S_IFBLK { // S_IFMT -- type of file
			return false, fmt.Errorf("not a block device: %s", d.Path)
		}

	case os.IsNotExist(err):
		return false, &os.PathError{"stat", d.Path, os.ErrNotExist}
	default:
		return false, err
	}

	return true, nil
}
