package kvmrun

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun/backend"
	"github.com/0xef53/kvmrun/kvmrun/backend/block"
	"github.com/0xef53/kvmrun/kvmrun/backend/iscsi"
	"github.com/0xef53/kvmrun/kvmrun/backend/nbd"
	"github.com/0xef53/kvmrun/kvmrun/internal/pool"
)

type CdromDriverType uint16

const (
	CdromDriverType_SCSI_CD CdromDriverType = iota + 1
	CdromDriverType_IDE_CD
)

func (t CdromDriverType) String() string {
	switch t {
	case CdromDriverType_SCSI_CD:
		return "scsi-cd"
	case CdromDriverType_IDE_CD:
		return "ide-cd"
	}

	return "UNKNOWN"
}

func (t CdromDriverType) HotPluggable() bool {
	switch t {
	case CdromDriverType_SCSI_CD:
		return true
	}

	return false
}

func CdromDriverTypeValue(s string) CdromDriverType {
	switch strings.ToLower(s) {
	case "scsi-cd":
		return CdromDriverType_SCSI_CD
	case "ide-cd":
		return CdromDriverType_IDE_CD
	}

	return DriverType_UNKNOWN
}

func DefaultCdromDriver() CdromDriverType {
	return CdromDriverType_IDE_CD
}

type CdromProperties struct {
	Name      string `json:"name"`
	Media     string `json:"media"`
	Driver    string `json:"driver"`
	Bootindex int    `json:"bootindex,omitempty"`
	Readonly  bool   `json:"readonly,omitempty"`
}

func (p *CdromProperties) Validate(strict bool) error {
	p.Name = strings.TrimSpace(p.Name)

	if len(p.Name) == 0 {
		return fmt.Errorf("empty cdrom name")
	}

	p.Driver = strings.TrimSpace(p.Driver)

	if len(p.Driver) == 0 {
		if strict {
			return fmt.Errorf("undefined cdrom driver")
		}

		p.Driver = DefaultCdromDriver().String()
	} else {
		if CdromDriverTypeValue(p.Driver) == DriverType_UNKNOWN && strict {
			return fmt.Errorf("unknown cdrom driver: %s", p.Driver)
		}
	}

	p.Media = strings.TrimSpace(p.Media)

	if p.Bootindex < 0 {
		return fmt.Errorf("invalid bootindex value: cannot be less than 0")
	}

	return nil
}

func NewCdromBackend(media string) (backend.DiskBackend, error) {
	switch {
	case strings.HasPrefix(media, "iscsi://"):
		return iscsi.New(media)
	case strings.HasPrefix(media, "nbd://"):
		return nbd.New(media)
	case strings.HasPrefix(media, "/dev/"):
		return block.New(media)
	}

	return nil, &backend.UnknownBackendError{Path: media}
}

type Cdrom struct {
	CdromProperties

	driver CdromDriverType

	MediaBackend backend.DiskBackend `json:"-"`
	QemuAddr     string              `json:"addr,omitempty"`
}

func NewCdrom(name, media string) (*Cdrom, error) {
	media = strings.TrimSpace(media)

	cd := new(Cdrom)

	cd.Name = name
	cd.Media = media

	cd.driver = DefaultCdromDriver()
	cd.CdromProperties.Driver = cd.driver.String()

	if len(media) > 0 {
		if be, err := NewCdromBackend(media); err == nil {
			cd.MediaBackend = be
		} else {
			return nil, err
		}
	}

	if err := cd.Validate(false); err != nil {
		return nil, err
	}

	return cd, nil
}

func (d *Cdrom) Copy() *Cdrom {
	v := Cdrom{CdromProperties: d.CdromProperties}

	v.driver = d.driver

	if d.MediaBackend != nil {
		v.MediaBackend = d.MediaBackend.Copy()
	}

	v.QemuAddr = d.QemuAddr

	return &v
}

func (d *Cdrom) Driver() CdromDriverType {
	return d.driver
}

func (d *Cdrom) QdevID() string {
	return "cdrom_" + d.Name
}

func (d *Cdrom) IsLocal() bool {
	return d.MediaBackend.IsLocal()
}

type CdromPool struct {
	pool.Pool
}

// Get returns a pointer to a Cdrom with name = devname.
func (p *CdromPool) Get(devname string) (cd *Cdrom) {
	err := p.Pool.GetAs(devname, &cd)
	if err == nil {
		return cd
	}

	return nil
}

// Exists returns true if a Cdrom with name = devname is in the pool.
// Otherwise returns false.
func (p *CdromPool) Exists(devname string) bool {
	return p.Pool.Exists(devname)
}

// Values returns all or specified cdroms from the pool.
func (p *CdromPool) Values(devnames ...string) []*Cdrom {
	values := make([]*Cdrom, 0, p.Len())

	for _, v := range p.Pool.Values(devnames...) {
		if d, ok := v.(*Cdrom); ok {
			values = append(values, d)
		}
	}

	return values
}

// Append appends a new Cdrom to the end of the pool.
func (p *CdromPool) Append(cd *Cdrom) error {
	return p.Pool.Append(cd.Name, cd, false)
}

// Insert inserts a new Cdrom into the pool at the given position.
func (p *CdromPool) Insert(cd *Cdrom, idx int) error {
	return p.Pool.Insert(cd.Name, cd, idx)
}

// Remove removes a Cdrom with name = devname from the pool.
func (p *CdromPool) Remove(devname string) (err error) {
	return p.Pool.Remove(devname)
}

func (p *CdromPool) UnmarshalJSON(data []byte) (err error) {
	var cdroms []*Cdrom

	if err := json.Unmarshal(data, &cdroms); err != nil {
		return err
	}

	for _, cd := range cdroms {
		cd.driver = CdromDriverTypeValue(cd.CdromProperties.Driver)

		if be, err := NewDiskBackend(cd.Media); err == nil {
			cd.MediaBackend = be
		}

		if err = p.Pool.Append(cd.Name, cd, false); err != nil {
			return err
		}
	}

	return nil
}
