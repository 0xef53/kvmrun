package kvmrun

import (
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun/backend"
	"github.com/0xef53/kvmrun/kvmrun/backend/block"
	"github.com/0xef53/kvmrun/kvmrun/backend/iscsi"
	"github.com/0xef53/kvmrun/kvmrun/backend/nbd"
)

var CdromDrivers = DevDrivers{
	DevDriver{"scsi-cd", true},
	DevDriver{"ide-cd", false},
}

type Cdrom struct {
	Name      string `json:"name"`
	Media     string `json:"media"`
	Driver    string `json:"driver"`
	Addr      string `json:"addr,omitempty"`
	Bootindex uint   `json:"bootindex,omitempty"`
	ReadOnly  bool   `json:"readonly,omitempty"`

	Backend backend.DiskBackend `json:"-"`
}

func NewCdrom(name, media string) (*Cdrom, error) {
	media = strings.TrimSpace(media)

	if len(media) == 0 {
		return nil, fmt.Errorf("media path cannot be empty")
	}

	d := Cdrom{
		Name:   name,
		Media:  media,
		Driver: "ide-cd",
	}

	b, err := NewCdromBackend(media)
	if err != nil {
		return nil, err
	}
	d.Backend = b

	return &d, nil
}

func (d Cdrom) QdevID() string {
	return "cdrom_" + d.Name
}

func (d Cdrom) IsLocal() bool {
	return d.Backend.IsLocal()
}

type CDPool []Cdrom

// Get returns a pointer to an element with Path == p.
func (p CDPool) Get(name string) *Cdrom {
	for k, _ := range p {
		if p[k].Name == name {
			return &p[k]
		}
	}

	return nil
}

// Exists returns true if an element with Path == p is present in the list.
// Otherwise returns false.
func (p CDPool) Exists(name string) bool {
	for _, d := range p {
		if d.Name == name {
			return true
		}
	}

	return false
}

// Append appends a new element to the end of the list.
func (p *CDPool) Append(d *Cdrom) {
	*p = append(*p, *d)
}

// Insert inserts a new element into the list at a given position.
func (p *CDPool) Insert(d *Cdrom, idx int) error {
	if idx < 0 {
		return fmt.Errorf("invalid disk index: %d", idx)
	}

	*p = append(*p, Cdrom{})
	copy((*p)[idx+1:], (*p)[idx:])
	(*p)[idx] = *d

	return nil
}

// Remove removes an element with Ifname == ifname from the list.
func (p *CDPool) Remove(name string) error {
	for idx, d := range *p {
		if d.Name == name {
			return (*p).RemoveN(idx)
		}
	}

	return fmt.Errorf("device not found: %s", p)
}

// RemoveN removes an element with Index == idx from the list.
func (p *CDPool) RemoveN(idx int) error {
	if !(idx >= 0 && idx <= len(*p)) {
		return fmt.Errorf("invalid device index: %d", idx)
	}

	switch {
	case idx == len(*p):
		*p = (*p)[:idx]
	default:
		*p = append((*p)[:idx], (*p)[idx+1:]...)
	}

	return nil
}

func NewCdromBackend(p string) (backend.DiskBackend, error) {
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
