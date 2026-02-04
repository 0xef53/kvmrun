package kvmrun

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun/internal/pool"

	"github.com/0xef53/go-tuntap"
	"github.com/vishvananda/netlink"
)

func ValidateLinkName(ifname string) error {
	if len(ifname) >= 3 && len(ifname) <= 16 {
		return nil
	}

	return fmt.Errorf("invalid link-name: min length = 3, max length = 16")
}

type NetDriverType uint16

const (
	NetDriverType_VIRTIO_NET_PCI NetDriverType = iota + 1
	NetDriverType_RTL8139
	NetDriverType_E1000
)

func (t NetDriverType) String() string {
	switch t {
	case NetDriverType_VIRTIO_NET_PCI:
		return "virtio-net-pci"
	case NetDriverType_RTL8139:
		return "rtl8139"
	case NetDriverType_E1000:
		return "e1000"
	}

	return "UNKNOWN"
}

func (t NetDriverType) HotPluggable() bool {
	switch t {
	case NetDriverType_VIRTIO_NET_PCI:
		return true
	}

	return false
}

func NetDriverTypeValue(s string) NetDriverType {
	switch strings.ToLower(s) {
	case "virtio-net-pci":
		return NetDriverType_VIRTIO_NET_PCI
	case "rtl8139":
		return NetDriverType_RTL8139
	case "e1000":
		return NetDriverType_E1000
	}

	return DriverType_UNKNOWN
}

func DefaultNetDriver() NetDriverType {
	return NetDriverType_VIRTIO_NET_PCI
}

type NetLinkState uint16

const (
	NetLinkState_UP NetLinkState = iota + 1
	NetLinkState_DOWN
)

func (s NetLinkState) String() string {
	switch s {
	case NetLinkState_UP:
		return "UP"
	case NetLinkState_DOWN:
		return "DOWN"
	}

	return "UNKNOWN"
}

func NetLinkStateValue(s string) NetLinkState {
	switch strings.ToLower(s) {
	case "up":
		return NetLinkState_UP
	case "down":
		return NetLinkState_DOWN
	}

	return 0
}

type NetIfaceProperties struct {
	Ifname    string `json:"ifname"`
	Driver    string `json:"driver"`
	HwAddr    string `json:"hwaddr"`
	Queues    int    `json:"queues"`
	Bootindex int    `json:"bootindex,omitempty"`
	Ifup      string `json:"ifup,omitempty"`
	Ifdown    string `json:"ifdown,omitempty"`
}

func (p *NetIfaceProperties) Validate(strict bool) error {
	p.Ifname = strings.TrimSpace(p.Ifname)

	if err := ValidateLinkName(p.Ifname); err != nil {
		return err
	}

	p.HwAddr = strings.TrimSpace(p.HwAddr)

	if len(p.HwAddr) == 0 {
		if v, err := GenHwAddr(); err == nil {
			p.HwAddr = v
		} else {
			return err
		}
	} else {
		if strict {
			if _, err := net.ParseMAC(p.HwAddr); err != nil {
				return err
			}
		}
	}

	p.Driver = strings.TrimSpace(p.Driver)

	if len(p.Driver) == 0 {
		if strict {
			return fmt.Errorf("undefined net interface driver")
		}

		p.Driver = DefaultNetDriver().String()
	} else {
		if NetDriverTypeValue(p.Driver) == DriverType_UNKNOWN && strict {
			return fmt.Errorf("unknown net interface driver: %s", p.Driver)
		}
	}

	if p.Queues < 0 {
		return fmt.Errorf("invalid queues value: cannot be less than 0")
	}

	if p.Bootindex < 0 {
		return fmt.Errorf("invalid bootindex value: cannot be less than 0")
	}

	p.Ifup = strings.TrimSpace(p.Ifup)
	p.Ifdown = strings.TrimSpace(p.Ifdown)

	if strict {
		for _, fname := range []string{p.Ifup, p.Ifdown} {
			if len(fname) > 0 {
				if _, err := os.Stat(fname); err != nil {
					if os.IsNotExist(err) {
						return err
					}
					return fmt.Errorf("failed to check %s: %w", fname, err)
				}
			}
		}
	}

	return nil
}

type NetIface struct {
	NetIfaceProperties

	driver NetDriverType

	QemuAddr string `json:"addr,omitempty"`
}

func NewNetIface(ifname, hwaddr string) (*NetIface, error) {
	n := new(NetIface)

	n.Ifname = ifname
	n.HwAddr = hwaddr

	n.driver = DefaultNetDriver()
	n.NetIfaceProperties.Driver = n.driver.String()

	if err := n.Validate(false); err != nil {
		return nil, err
	}

	return n, nil
}

func (n *NetIface) Copy() *NetIface {
	v := NetIface{NetIfaceProperties: n.NetIfaceProperties}

	v.QemuAddr = n.QemuAddr

	v.driver = n.driver

	return &v
}

func (n *NetIface) Driver() NetDriverType {
	return n.driver
}

func (iface *NetIface) QdevID() string {
	return fmt.Sprintf("net_%s", iface.Ifname)
}

type NetIfacePool struct {
	pool.Pool
}

// Get returns a pointer to a NetIface with name = ifname.
func (p *NetIfacePool) Get(ifname string) (n *NetIface) {
	err := p.Pool.GetAs(ifname, &n)
	if err == nil {
		return n
	}

	return nil
}

// Exists returns true if a NetIface with name = ifname is in the pool.
// Otherwise returns false.
func (p *NetIfacePool) Exists(ifname string) bool {
	return p.Pool.Exists(ifname)
}

// Values returns all or specified NetIfaces from the pool.
func (p *NetIfacePool) Values(ifnames ...string) []*NetIface {
	values := make([]*NetIface, 0, p.Len())

	for _, v := range p.Pool.Values(ifnames...) {
		if d, ok := v.(*NetIface); ok {
			values = append(values, d)
		}
	}

	return values
}

// Append appends a new NetIface to the end of the pool.
func (p *NetIfacePool) Append(n *NetIface) error {
	return p.Pool.Append(n.Ifname, n, false)
}

// Insert inserts a new NetIface into the pool at the given position.
func (p *NetIfacePool) Insert(n *NetIface, idx int) error {
	return p.Pool.Insert(n.Ifname, n, idx)
}

// Remove removes a NetIface with name = ifname from the pool.
func (p *NetIfacePool) Remove(ifname string) (err error) {
	return p.Pool.Remove(ifname)
}

func (p *NetIfacePool) UnmarshalJSON(data []byte) (err error) {
	var ifaces []*NetIface

	if err := json.Unmarshal(data, &ifaces); err != nil {
		return err
	}

	for _, n := range ifaces {
		n.driver = NetDriverTypeValue(n.NetIfaceProperties.Driver)

		if err = p.Pool.Append(n.Ifname, n, false); err != nil {
			return err
		}
	}

	return nil
}

// AddTapInterface creates a new tap interface with name = ifname and owner = uid.
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

// DelTapInterface destroys an existing tap interface with name = ifname.
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
