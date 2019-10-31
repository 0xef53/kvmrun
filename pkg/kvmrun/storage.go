package kvmrun

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/0xef53/kvmrun/pkg/block"
	"github.com/0xef53/kvmrun/pkg/iscsi"
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
	Backend         DiskBackend `json:"-"`
	Path            string      `json:"path"`
	Driver          string      `json:"driver"`
	IopsRd          int         `json:"iops_rd"`
	IopsWr          int         `json:"iops_wr"`
	Addr            string      `json:"addr,omitempty"`
	Bootindex       uint        `json:"bootindex,omitempty"`
	QemuVirtualSize uint64      `json:"-"`
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

type Disks []Disk

// // Clone returns a duplicate of a Disks object (deep copy).
func (dd Disks) Clone() Disks {
	x := make(Disks, 0, len(dd))

	for _, disk := range dd {
		tmpDisk := disk

		switch v := disk.Backend.(type) {
		case BlkDisk, ISCSIDisk:
			tmpDisk.Backend = v
		default:
			panic(fmt.Sprintf("unknown backend type: %T", v))
		}

		x = append(x, tmpDisk)
	}

	return x
}

// Get returns a pointer to an element with Path == p.
func (dd Disks) Get(p string) *Disk {
	for k, _ := range dd {
		if dd[k].Path == p || dd[k].BaseName() == p {
			return &dd[k]
		}
	}
	return nil
}

// Exists returns true if an element with Path == p is present in the list.
// Otherwise returns false.
func (dd Disks) Exists(p string) bool {
	for _, d := range dd {
		if d.Path == p || d.BaseName() == p {
			return true
		}
	}
	return false
}

// Append appends a new element to the end of the list.
func (dd *Disks) Append(d *Disk) {
	*dd = append(*dd, *d)
}

// Insert inserts a new element into the list at a given position.
func (dd *Disks) Insert(d *Disk, idx int) error {
	if idx < 0 {
		return fmt.Errorf("Invalid disk index: %d", idx)
	}
	*dd = append(*dd, Disk{})
	copy((*dd)[idx+1:], (*dd)[idx:])
	(*dd)[idx] = *d
	return nil
}

// Remove removes an element with Ifname == ifname from the list.
func (dd *Disks) Remove(p string) error {
	for idx, d := range *dd {
		if d.Path == p || d.BaseName() == p {
			return (*dd).RemoveN(idx)
		}
	}
	return fmt.Errorf("Disk not found: %s", p)
}

// RemoveN removes an element with Index == idx from the list.
func (dd *Disks) RemoveN(idx int) error {
	if !(idx >= 0 && idx <= len(*dd)) {
		return fmt.Errorf("Invalid disk index: %d", idx)
	}
	switch {
	case idx == len(*dd):
		*dd = (*dd)[:idx]
	default:
		*dd = append((*dd)[:idx], (*dd)[idx+1:]...)
	}
	return nil
}

type DiskBackend interface {
	BaseName() string
	QdevID() string
	Size() (uint64, error)
	IsLocal() bool
	IsAvailable() (bool, error)
}

func NewDiskBackend(p string) (DiskBackend, error) {
	switch {
	case strings.HasPrefix(p, "iscsi://"):
		b, err := NewISCSIDisk(p)
		if err != nil {
			return nil, err
		}
		return *b, nil
	case strings.HasPrefix(p, "/dev/"):
		b, err := NewBlkDisk(p)
		if err != nil {
			return nil, err
		}
		return *b, nil
	}
	return nil, fmt.Errorf("Unable to determine the type of %s", p)
}

type BlkDisk struct {
	Path string
}

func NewBlkDisk(p string) (*BlkDisk, error) {
	return &BlkDisk{Path: p}, nil
}

func (d BlkDisk) QdevID() string {
	return "blk_" + d.BaseName()
}

func (d BlkDisk) BaseName() string {
	return filepath.Base(d.Path)
}

func (d BlkDisk) Size() (uint64, error) {
	return block.BlkGetSize64(d.Path)
}

func (d BlkDisk) IsLocal() bool {
	return true
}

func (d BlkDisk) IsAvailable() (bool, error) {
	var st unix.Stat_t

	switch err := unix.Stat(d.Path, &st); {
	case err == nil:
		if (st.Mode & unix.S_IFMT) != unix.S_IFBLK { // S_IFMT -- type of file
			return false, fmt.Errorf("Not a block device: %s", d.Path)
		}

	case os.IsNotExist(err):
		return false, &os.PathError{"stat", d.Path, os.ErrNotExist}
	default:
		return false, err
	}

	return true, nil
}

// We support iscsi url's on the form
// iscsi://[<username>%<password>@]<host>[:<port>]/<targetname>/<lun>
// E.g.:
// iscsi://client%secret@192.168.0.254/iqn.2018-02.ru.netangels.cvds:mailstorage/0
type ISCSIDisk struct {
	Path string
	Url  iscsi.URL
}

func NewISCSIDisk(p string) (*ISCSIDisk, error) {
	u, err := iscsi.ParseFullURL(p)
	if err != nil {
		return nil, err
	}

	d := ISCSIDisk{
		Path: p,
		Url:  *u,
	}

	return &d, nil
}

func (d ISCSIDisk) QdevID() string {
	return "blk_" + d.Url.UniqueName
}

func (d ISCSIDisk) BaseName() string {
	return d.Url.UniqueName
}

func (d ISCSIDisk) Size() (uint64, error) {
	return 0, ErrNotImplemented
}

func (d ISCSIDisk) IsLocal() bool {
	return false
}

func (d ISCSIDisk) IsAvailable() (bool, error) {
	return true, nil
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
