package iscsi

import (
	"github.com/0xef53/kvmrun/kvmrun/backend"
)

type Device struct {
	Path string
	URI  *URI
}

func New(p string) (*Device, error) {
	u, err := ParseURI(p)
	if err != nil {
		return nil, err
	}

	d := Device{
		Path: p,
		URI:  u,
	}

	return &d, nil
}

func (d *Device) QdevID() string {
	return "blk_" + d.URI.UniqueName
}

func (d *Device) FullPath() string {
	return d.Path
}

func (d *Device) BaseName() string {
	return d.URI.UniqueName
}

func (d *Device) Size() (uint64, error) {
	return 0, backend.ErrNotImplemented
}

func (d *Device) IsLocal() bool {
	return false
}

func (d *Device) IsAvailable() (bool, error) {
	return true, nil
}

func (d *Device) Copy() backend.DiskBackend {
	_uri := *d.URI

	return &Device{
		Path: d.Path,
		URI:  &_uri,
	}
}
