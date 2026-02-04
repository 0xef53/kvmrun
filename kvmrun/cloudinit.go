package kvmrun

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun/backend"
	"github.com/0xef53/kvmrun/kvmrun/backend/block"
	"github.com/0xef53/kvmrun/kvmrun/backend/file"
)

type CloudInitDriverType uint16

const (
	CloudInitDriverType_IDE_CD CloudInitDriverType = iota + 1
	CloudInitDriverType_FLOPPY
)

func (t CloudInitDriverType) String() string {
	switch t {
	case CloudInitDriverType_IDE_CD:
		return "ide-cd"
	case CloudInitDriverType_FLOPPY:
		return "floppy"
	}

	return "UNKNOWN"
}

func (t CloudInitDriverType) HotPluggable() bool {
	return false
}

func CloudInitDriverTypeValue(s string) CloudInitDriverType {
	switch strings.ToLower(s) {
	case "ide-cd":
		return CloudInitDriverType_IDE_CD
	case "floppy":
		return CloudInitDriverType_FLOPPY
	}

	return DriverType_UNKNOWN
}

func DefaultCloudInitDriver() CloudInitDriverType {
	return CloudInitDriverType_IDE_CD
}

type CloudInitDriveProperties struct {
	Media  string `json:"path,omitempty"`
	Driver string `json:"driver,omitempty"`
}

func (p *CloudInitDriveProperties) Validate(strict bool) error {
	p.Media = strings.TrimSpace(p.Media)

	if len(p.Media) == 0 {
		return fmt.Errorf("empty cloud-init media path")
	}

	p.Driver = strings.TrimSpace(p.Driver)

	if len(p.Driver) == 0 {
		if strict {
			return fmt.Errorf("undefined cloud-init driver")
		}

		p.Driver = DefaultCloudInitDriver().String()
	} else {
		if CloudInitDriverTypeValue(p.Driver) == DriverType_UNKNOWN && strict {
			return fmt.Errorf("unknown cloud-init driver: %s", p.Driver)
		}
	}

	return nil
}

func NewCloudInitDriveBackend(media string) (backend.DiskBackend, error) {
	switch {
	case strings.HasPrefix(media, "iscsi://"):
		return nil, &backend.UnknownBackendError{Path: media}
	case strings.HasPrefix(media, "nbd://"):
		return nil, &backend.UnknownBackendError{Path: media}
	case strings.HasPrefix(media, "/dev/"):
		return block.New(media)
	}

	return file.New(media)
}

type CloudInitDrive struct {
	CloudInitDriveProperties

	driver CloudInitDriverType

	Backend backend.DiskBackend `json:"-"`
}

func NewCloudInitDrive(media string) (*CloudInitDrive, error) {
	media = strings.TrimSpace(media)

	if len(media) == 0 {
		return nil, fmt.Errorf("empty cloud-init media path")
	}

	d := new(CloudInitDrive)

	d.Media = media

	d.driver = DefaultCloudInitDriver()
	d.CloudInitDriveProperties.Driver = d.driver.String()

	if be, err := NewCloudInitDriveBackend(media); err == nil {
		d.Backend = be
	} else {
		return nil, err
	}

	if err := d.Validate(false); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *CloudInitDrive) Copy() *CloudInitDrive {
	v := CloudInitDrive{CloudInitDriveProperties: d.CloudInitDriveProperties}

	v.driver = d.driver

	if d.Backend != nil {
		v.Backend = d.Backend.Copy()
	}

	return &v
}

func (d *CloudInitDrive) Driver() CloudInitDriverType {
	return d.driver
}

func (d *CloudInitDrive) QdevID() string {
	return "cidata"
}

func (d *CloudInitDrive) IsLocal() bool {
	return d.Backend.IsLocal()
}

func (d *CloudInitDrive) UnmarshalJSON(data []byte) (err error) {
	opts := CloudInitDriveProperties{}

	if err := json.Unmarshal(data, &opts); err != nil {
		return err
	}

	d.Media = opts.Media
	d.CloudInitDriveProperties.Driver = opts.Driver

	d.driver = CloudInitDriverTypeValue(opts.Driver)

	if be, err := NewCloudInitDriveBackend(opts.Media); err == nil {
		d.Backend = be
	}

	return nil
}
