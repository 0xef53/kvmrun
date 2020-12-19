package kvmrun

import (
	"crypto/rand"
	"fmt"
	"strings"
	"syscall"

	"github.com/0xef53/go-tuntap"
)

var NetDrivers = DevDrivers{
	DevDriver{"virtio-net-pci", true},
	DevDriver{"rtl8139", true},
	DevDriver{"e1000", true},
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

func (iface *NetIface) QemuCommandArgs() []string {
	netdevOpts := []string{
		"tap",
		fmt.Sprintf("ifname=%s", iface.Ifname),
		fmt.Sprintf("id=%s", iface.Ifname),
		"vhost=on",
		fmt.Sprintf("script=%s", VMNETINIT),
		"downscript=no",
	}

	deviceOpts := []string{
		iface.Driver,
		fmt.Sprintf("netdev=%s", iface.Ifname),
		fmt.Sprintf("id=%s", iface.QdevID()),
		fmt.Sprintf("mac=%s", iface.HwAddr),
	}

	if NetDrivers.HotPluggable(iface.Driver) {
		deviceOpts = append(deviceOpts, "bus=pci.0")
		if iface.Addr != "" {
			deviceOpts = append(deviceOpts, fmt.Sprintf("addr=%s", iface.Addr))
		}
	}

	if iface.Bootindex > 0 {
		deviceOpts = append(deviceOpts, fmt.Sprintf("bootindex=%d", iface.Bootindex))
	}

	return []string{"-netdev", strings.Join(netdevOpts, ","), "-device", strings.Join(deviceOpts, ",")}
}

type NetIfaces []NetIface

// Get returns a pointer to an element with Ifname == ifname.
func (nn NetIfaces) Get(ifname string) *NetIface {
	for k, _ := range nn {
		if nn[k].Ifname == ifname {
			return &nn[k]
		}
	}
	return nil
}

// Exists returns true if an element with Ifname == ifname is present in the list.
// Otherwise returns false.
func (nn NetIfaces) Exists(ifname string) bool {
	for _, iface := range nn {
		if iface.Ifname == ifname {
			return true
		}
	}
	return false
}

// Append appends a new element to the end of the list.
func (nn *NetIfaces) Append(iface *NetIface) {
	*nn = append(*nn, *iface)
}

// Insert inserts a new element into the list at a given position.
func (nn *NetIfaces) Insert(iface *NetIface, idx int) error {
	if idx < 0 {
		return fmt.Errorf("Invalid interface index: %d", idx)
	}
	*nn = append(*nn, NetIface{})
	copy((*nn)[idx+1:], (*nn)[idx:])
	(*nn)[idx] = *iface
	return nil
}

// Remove removes an element with Ifname == ifname from the list.
func (nn *NetIfaces) Remove(ifname string) error {
	for idx, iface := range *nn {
		if iface.Ifname == ifname {
			return (*nn).RemoveN(idx)
		}
	}
	return fmt.Errorf("Network interface not found: %s", ifname)
}

// RemoveN removes an element with Index == idx from the list.
func (nn *NetIfaces) RemoveN(idx int) error {
	if !(idx >= 0 && idx <= len(*nn)) {
		return fmt.Errorf("Invalid interface index: %d", idx)
	}
	switch {
	case idx == len(*nn):
		*nn = (*nn)[:idx]
	default:
		*nn = append((*nn)[:idx], (*nn)[idx+1:]...)
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
	if err := tuntap.AddTapInterface(ifname, uid, -1, uint16(flags), true); err != nil {
		return err
	}
	return nil
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
