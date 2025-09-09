package kvmrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun/backend"
	"github.com/0xef53/kvmrun/kvmrun/backend/block"
	"github.com/0xef53/kvmrun/kvmrun/backend/iscsi"
	"github.com/0xef53/kvmrun/kvmrun/backend/nbd"
	"github.com/0xef53/kvmrun/kvmrun/internal/pool"
)

type DiskDriverType uint16

const (
	DiskDriverType_VIRTIO_BLK_PCI DiskDriverType = iota + 1
	DiskDriverType_SCSI_HD
	DiskDriverType_IDE_HD
)

func (t DiskDriverType) String() string {
	switch t {
	case DiskDriverType_VIRTIO_BLK_PCI:
		return "virtio-blk-pci"
	case DiskDriverType_SCSI_HD:
		return "scsi-hd"
	case DiskDriverType_IDE_HD:
		return "ide-hd"
	}

	return "UNKNOWN"
}

func (t DiskDriverType) HotPluggable() bool {
	switch t {
	case DiskDriverType_VIRTIO_BLK_PCI:
		fallthrough
	case DiskDriverType_SCSI_HD:
		return true
	}

	return false
}

func DiskDriverTypeValue(s string) DiskDriverType {
	switch strings.ToLower(s) {
	case "virtio-blk-pci":
		return DiskDriverType_VIRTIO_BLK_PCI
	case "scsi-hd":
		return DiskDriverType_SCSI_HD
	case "ide-hd":
		return DiskDriverType_IDE_HD
	}

	return DriverType_UNKNOWN
}

func DefaultDiskDriver() DiskDriverType {
	return DiskDriverType_VIRTIO_BLK_PCI
}

type DiskProperties struct {
	Path      string `json:"path"`
	Driver    string `json:"driver"`
	IopsRd    int    `json:"iops_rd"`
	IopsWr    int    `json:"iops_wr"`
	Bootindex int    `json:"bootindex,omitempty"`
}

func (p *DiskProperties) Validate(strict bool) error {
	p.Path = strings.TrimSpace(p.Path)

	if len(p.Path) == 0 {
		return fmt.Errorf("empty disk path")
	}

	p.Driver = strings.TrimSpace(p.Driver)

	if len(p.Driver) == 0 {
		if strict {
			return fmt.Errorf("undefined disk driver")
		}

		p.Driver = DefaultDiskDriver().String()
	} else {
		if DiskDriverTypeValue(p.Driver) == DriverType_UNKNOWN && strict {
			return fmt.Errorf("unknown disk driver: %s", p.Driver)
		}
	}

	switch {
	case p.IopsRd < 0 || p.IopsWr < 0:
		return fmt.Errorf("invalid iops value: cannot be less than 0")
	case p.Bootindex < 0:
		return fmt.Errorf("invalid bootindex value: cannot be less than 0")
	}

	return nil
}

func NewDiskBackend(p string) (backend.DiskBackend, error) {
	p = strings.TrimSpace(p)

	switch {
	case strings.HasPrefix(p, "iscsi://"):
		return iscsi.New(p)
	case strings.HasPrefix(p, "nbd://"):
		return nbd.New(p)
	case strings.HasPrefix(p, "/dev/"):
		return block.New(p)
	}

	return nil, &backend.UnknownBackendError{Path: p}
}

type Disk struct {
	DiskProperties

	driver DiskDriverType

	Backend         backend.DiskBackend `json:"-"`
	QemuAddr        string              `json:"addr,omitempty"`
	QemuVirtualSize uint64              `json:"-"`
	HasBitmap       bool                `json:"-"`
}

func NewDisk(p string) (*Disk, error) {
	d := new(Disk)

	d.Path = p

	d.driver = DefaultDiskDriver()
	d.DiskProperties.Driver = d.driver.String()

	if be, err := NewDiskBackend(p); err == nil {
		d.Backend = be
	} else {
		return nil, err
	}

	if err := d.Validate(false); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Disk) Copy() *Disk {
	v := Disk{DiskProperties: d.DiskProperties}

	v.driver = d.driver

	if d.Backend != nil {
		v.Backend = d.Backend.Copy()
	}

	v.QemuAddr = d.QemuAddr
	v.QemuVirtualSize = d.QemuVirtualSize
	v.HasBitmap = d.HasBitmap

	return &v
}

func (d *Disk) Driver() DiskDriverType {
	return d.driver
}

func (d *Disk) QdevID() string {
	return d.Backend.QdevID()
}

func (d *Disk) BaseName() string {
	return d.Backend.BaseName()
}

func (d *Disk) IsLocal() bool {
	return d.Backend.IsLocal()
}

func (d *Disk) IsAvailable() (bool, error) {
	return d.Backend.IsAvailable()
}

type DiskPool struct {
	pool.Pool
}

// Get returns a pointer to a Disk with name/path = diskname.
func (p *DiskPool) Get(diskname string) (d *Disk) {
	err := p.Pool.GetAs(diskname, &d)
	if err == nil {
		return d
	}

	// Perhaps diskname is a fully qualified name.
	// If so, try to find by backend BaseName().

	if errors.Is(err, pool.ErrNotFound) {
		if be, err := NewDiskBackend(diskname); err == nil {
			if err := p.Pool.GetAs(be.BaseName(), &d); err == nil {
				return d
			}
		}
	}

	return nil
}

// Exists returns true if a Disk with name/path = diskname is in the pool.
// Otherwise returns false.
func (p *DiskPool) Exists(diskname string) bool {
	if p.Pool.Exists(diskname) {
		return true
	}

	// Perhaps diskname is a fully qualified name.
	// If so, try to check by backend BaseName().

	if be, err := NewDiskBackend(diskname); err == nil {
		return p.Pool.Exists(be.BaseName())
	}

	return false
}

// Values returns all or specified disks from the pool.
func (p *DiskPool) Values(disknames ...string) []*Disk {
	all := make([]*Disk, 0, p.Len())

	for _, v := range p.Pool.Values() {
		if d, ok := v.(*Disk); ok {
			all = append(all, d)
		}
	}

	if len(disknames) > 0 {
		tmp := make(map[string]struct{})

		for _, dname := range disknames {
			tmp[dname] = struct{}{}
		}

		valid := make([]*Disk, 0, len(disknames))

		for _, d := range all {
			if _, ok := tmp[d.Path]; ok {
				valid = append(valid, d)
			} else if _, ok := tmp[d.Backend.BaseName()]; ok {
				valid = append(valid, d)
			}
		}

		return valid
	}

	return all
}

// Append appends a new Disk to the end of the pool.
func (p *DiskPool) Append(d *Disk) error {
	var diskname string

	if d.Backend == nil {
		diskname = d.Path
	} else {
		diskname = d.Backend.BaseName()
	}

	return p.Pool.Append(diskname, d, false)
}

// Insert inserts a new Disk into the pool at the given position.
func (p *DiskPool) Insert(d *Disk, idx int) error {
	var diskname string

	if d.Backend == nil {
		diskname = d.Path
	} else {
		diskname = d.Backend.BaseName()
	}

	return p.Pool.Insert(diskname, d, idx)
}

// Remove removes a Disk with name/path = diskname from the pool.
func (p *DiskPool) Remove(diskname string) (err error) {
	err = p.Pool.Remove(diskname)

	// Perhaps diskname is a fully qualified name.
	// If so, try to remove by backend BaseName() as well.

	if err != nil {
		if be, _err := NewDiskBackend(diskname); _err == nil {
			err = p.Pool.Remove(be.BaseName())
		}
	}

	return err
}

func (p *DiskPool) UnmarshalJSON(data []byte) (err error) {
	var disks []*Disk

	if err := json.Unmarshal(data, &disks); err != nil {
		return err
	}

	for _, d := range disks {
		d.driver = DiskDriverTypeValue(d.DiskProperties.Driver)

		var err error

		if be, _err := NewDiskBackend(d.Path); _err == nil {
			d.Backend = be

			err = p.Pool.Append(be.BaseName(), d, false)
		} else {
			err = p.Pool.Append(d.Path, d, false)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

type SCSIBusInfo struct {
	Type string `json:"type"`
	Addr string `json:"addr,omitempty"`
}

func ParseSCSIAddr(s string) (string, string, string) {
	// Format: scsi0:0x10/1
	var busName, busAddr, lun string

	parseBusNameAddr := func(x string) (string, string) {
		var name, addr string

		parts := strings.Split(x, ":")

		if len(parts[0]) > 0 {
			name = parts[0]
		} else {
			name = "scsi0"
		}

		if len(parts) > 1 && len(parts[1]) > 0 {
			addr = parts[1]
		}

		return name, addr
	}

	parts := strings.Split(s, "/")

	busName, busAddr = parseBusNameAddr(parts[0])

	if len(parts) > 1 && len(parts[1]) > 0 {
		lun = parts[1]
	}

	return busName, busAddr, lun
}
