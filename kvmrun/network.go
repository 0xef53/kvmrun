package kvmrun

import (
	"crypto/rand"
	"fmt"

	"github.com/0xef53/go-tuntap"
	"github.com/vishvananda/netlink"
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
	Queues    int    `json:"queues"`
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
	for k := range p {
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
func AddTapInterface(ifname string, uid int, mq bool) error {
	// Linux supports multiqueue tuntap from version 3.8
	// https://www.kernel.org/doc/Documentation/networking/tuntap.txt
	// (3.3 Multiqueue tuntap interface)

	link := &netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{Name: ifname},
		Mode:      netlink.TUNTAP_MODE_TAP,
		Flags:     netlink.TUNTAP_TUN_EXCL | netlink.TUNTAP_NO_PI,
		Owner:     1024,
	}

	if mq {
		link.Flags |= netlink.TUNTAP_MULTI_QUEUE
	} else {
		link.Flags |= netlink.TUNTAP_ONE_QUEUE
	}

	features, err := tuntap.GetFeatures()
	if err != nil {
		return err
	}

	if (features & uint16(netlink.TUNTAP_VNET_HDR)) != 0 {
		link.Flags |= netlink.TUNTAP_VNET_HDR
	}

	return netlink.LinkAdd(link)
}

// DelTapInterface destroys an existing tap interface with Name == ifname.
func DelTapInterface(ifname string) error {
	link, err := netlink.LinkByName(ifname)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return nil
		}
		return fmt.Errorf("netlink: %s", err)
	}

	return netlink.LinkDel(link)
}

// SetInterfaceUp changes the state of a given interface to UP.
func SetInterfaceUp(ifname string) error {
	link := &netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{Name: ifname},
		Mode:      netlink.TUNTAP_MODE_TAP,
	}

	return netlink.LinkSetUp(link)
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
