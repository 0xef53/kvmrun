package kvmrun

import (
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun/backend"
	"github.com/0xef53/kvmrun/kvmrun/backend/block"
	"github.com/0xef53/kvmrun/kvmrun/backend/file"
)

var CloudInitDrivers = DevDrivers{
	DevDriver{"ide-cd", false},
	DevDriver{"floppy", false},
}

// CloudInitDrive contains a path to the cloud-init drive.
type CloudInitDrive struct {
	Media  string `json:"path,omitempty"`
	Driver string `json:"driver,omitempty"`

	Backend backend.DiskBackend `json:"-"`
}

func NewCloudInitDrive(media string) (*CloudInitDrive, error) {
	media = strings.TrimSpace(media)

	if len(media) == 0 {
		return nil, fmt.Errorf("media path cannot be empty")
	}

	b, err := NewCloudInitDriveBackend(media)
	if err != nil {
		return nil, err
	}

	if ok, err := b.IsAvailable(); err == nil {
		if !ok {
			return nil, fmt.Errorf("not available: %s", b.FullPath())
		}
	} else {
		return nil, err
	}

	return &CloudInitDrive{
		Media:   b.FullPath(),
		Driver:  "ide-cd",
		Backend: b,
	}, nil
}

func (d *CloudInitDrive) QdevID() string {
	return "cidata"
}

func (d *CloudInitDrive) IsLocal() bool {
	return d.Backend.IsLocal()
}

func NewCloudInitDriveBackend(p string) (backend.DiskBackend, error) {
	switch {
	case strings.HasPrefix(p, "iscsi://"):
		return nil, &backend.UnknownBackendError{Path: p}
	case strings.HasPrefix(p, "nbd://"):
		return nil, &backend.UnknownBackendError{Path: p}
	case strings.HasPrefix(p, "/dev/"):
		return block.New(p)
	}

	return file.New(p)
}
