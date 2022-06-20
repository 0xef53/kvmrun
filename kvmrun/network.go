package kvmrun

import (
	"crypto/rand"
	"fmt"
	"syscall"

	"github.com/0xef53/go-tuntap"
)

var NetDrivers = DevDrivers{
	DevDriver{"virtio-net-pci", true},
	DevDriver{"rtl8139", false},
	DevDriver{"e1000", false},
}

type NetIface struct {
	Ifname    string `json:"ifname"`
	Driver    string `json:"driver"`
	HwAddr    string `json:"hwaddr"`
	Addr      string `json:"addr,omitempty"`
	Bootindex int    `json:"bootindex,omitempty"`
	Ifup      string `json:"ifup,omitempty"`
	Ifdown    string `json:"ifdown,omitempty"`
}

func (iface *NetIface) QdevID() string {
	return fmt.Sprintf("net_%s", iface.Ifname)
}

type NetifPool []NetIface

// Get returns a pointer to an element with Ifname == ifname.
func (p NetifPool) Get(ifname string) *NetIface {
	for k, _ := range p {
		if p[k].Ifname == ifname {
			return &p[k]
		}
	}

	return nil
}

// Exists returns true if an element with Ifname == ifname is present in the list.
// Otherwise returns false.
func (p NetifPool) Exists(ifname string) bool {
	for _, iface := range p {
		if iface.Ifname == ifname {
			return true
		}
	}

	return false
}

// Append appends a new element to the end of the list.
func (p *NetifPool) Append(iface *NetIface) {
	*p = append(*p, *iface)
}

// Insert inserts a new element into the list at a given position.
func (p *NetifPool) Insert(iface *NetIface, idx int) error {
	if idx < 0 {
		return fmt.Errorf("invalid interface index: %d", idx)
	}

	*p = append(*p, NetIface{})
	copy((*p)[idx+1:], (*p)[idx:])
	(*p)[idx] = *iface

	return nil
}

// Remove removes an element with Ifname == ifname from the list.
func (p *NetifPool) Remove(ifname string) error {
	for idx, iface := range *p {
		if iface.Ifname == ifname {
			return (*p).RemoveN(idx)
		}
	}

	return fmt.Errorf("network interface not found: %s", ifname)
}

// RemoveN removes an element with Index == idx from the list.
func (p *NetifPool) RemoveN(idx int) error {
	if !(idx >= 0 && idx <= len(*p)) {
		return fmt.Errorf("invalid interface index: %d", idx)
	}

	switch {
	case idx == len(*p):
		*p = (*p)[:idx]
	default:
		*p = append((*p)[:idx], (*p)[idx+1:]...)
	}

	return nil
}

// AddTapInterface creates a new tap interface with Name == ifname and owner == uid.
func AddTapInterface(ifname string, uid int) error {
	// Linux supports multiqueue tuntap from version 3.8
	// https://www.kernel.org/doc/Documentation/networking/tuntap.txt
	// (3.3 Multiqueue tuntap interface)
	flags := syscall.IFF_NO_PI | syscall.IFF_ONE_QUEUE

	features, err := tuntap.GetFeatures()
	if err != nil {
		return err
	}

	if (features & syscall.IFF_VNET_HDR) != 0 {
		flags |= syscall.IFF_VNET_HDR
	}

	return tuntap.AddTapInterface(ifname, uid, -1, uint16(flags), true)
}

// DelTapInterface destroys an existing tap interface with Name == ifname.
func DelTapInterface(ifname string) error {
	return tuntap.DelTapInterface(ifname)
}

// SetInterfaceUp changes the state of a given interface to UP.
func SetInterfaceUp(ifname string) error {
	return tuntap.SetInterfaceUp(ifname)
}

// GenHwAddr generates a random hardware address with Linux KVM prefix 54:52:00.
func GenHwAddr() (string, error) {
	buf := make([]byte, 3)

	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("54:52:00:%02x:%02x:%02x", buf[0], buf[1], buf[2]), nil
}
