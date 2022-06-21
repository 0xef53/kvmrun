package kvmrun

import (
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun/backend"
	"github.com/0xef53/kvmrun/kvmrun/backend/block"
	"github.com/0xef53/kvmrun/kvmrun/backend/iscsi"
	"github.com/0xef53/kvmrun/kvmrun/backend/nbd"
)

var DiskDrivers = DevDrivers{
	DevDriver{"virtio-blk-pci", true},
	DevDriver{"scsi-hd", true},
	DevDriver{"ide-hd", false},
}

type SCSIBusInfo struct {
	Type string `json:"type"`
	Addr string `json:"addr,omitempty"`
}

type Disk struct {
	Backend         backend.DiskBackend `json:"-"`
	Path            string              `json:"path"`
	Driver          string              `json:"driver"`
	IopsRd          int                 `json:"iops_rd"`
	IopsWr          int                 `json:"iops_wr"`
	Addr            string              `json:"addr,omitempty"`
	Bootindex       uint                `json:"bootindex,omitempty"`
	QemuVirtualSize uint64              `json:"-"`
	HasBitmap       bool                `json:"-"`
}

func NewDisk(p string) (*Disk, error) {
	d := Disk{
		Path:   p,
		Driver: "virtio-blk-pci",
	}

	b, err := NewDiskBackend(p)
	if err != nil {
		return nil, err
	}
	d.Backend = b

	return &d, nil
}

func (d Disk) QdevID() string {
	return d.Backend.QdevID()
}

func (d Disk) BaseName() string {
	return d.Backend.BaseName()
}

func (d Disk) IsLocal() bool {
	return d.Backend.IsLocal()
}

func (d Disk) IsAvailable() (bool, error) {
	return d.Backend.IsAvailable()
}

type DiskPool []Disk

// Get returns a pointer to an element with Path == p.
func (p DiskPool) Get(dpath string) *Disk {
	for k := range p {
		if p[k].Path == dpath || p[k].BaseName() == dpath {
			return &p[k]
		}
	}

	return nil
}

// Exists returns true if an element with Path == p is present in the list.
// Otherwise returns false.
func (p DiskPool) Exists(dpath string) bool {
	for _, d := range p {
		if d.Path == dpath || d.BaseName() == dpath {
			return true
		}
	}

	return false
}

// Append appends a new element to the end of the list.
func (p *DiskPool) Append(d *Disk) {
	*p = append(*p, *d)
}

// Insert inserts a new element into the list at a given position.
func (p *DiskPool) Insert(d *Disk, idx int) error {
	if idx < 0 {
		return fmt.Errorf("invalid disk index: %d", idx)
	}

	*p = append(*p, Disk{})
	copy((*p)[idx+1:], (*p)[idx:])
	(*p)[idx] = *d

	return nil
}

// Remove removes an element with Ifname == ifname from the list.
func (p *DiskPool) Remove(dpath string) error {
	for idx, d := range *p {
		if d.Path == dpath || d.BaseName() == dpath {
			return (*p).RemoveN(idx)
		}
	}

	return fmt.Errorf("disk not found: %s", p)
}

// RemoveN removes an element with Index == idx from the list.
func (p *DiskPool) RemoveN(idx int) error {
	if !(idx >= 0 && idx <= len(*p)) {
		return fmt.Errorf("invalid disk index: %d", idx)
	}

	switch {
	case idx == len(*p):
		*p = (*p)[:idx]
	default:
		*p = append((*p)[:idx], (*p)[idx+1:]...)
	}

	return nil
}

func NewDiskBackend(p string) (backend.DiskBackend, error) {
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
