package network

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

type BridgeDeviceAttrs struct {
	Ifname string
	MTU    uint32
}

func ConfigureBridgePort(linkname string, attrs *BridgeDeviceAttrs) error {
	brLink, err := netlink.LinkByName(attrs.Ifname)

	switch err.(type) {
	case nil:
		if _, ok := brLink.(*netlink.Bridge); !ok {
			return fmt.Errorf("not a bridge device: %s", attrs.Ifname)
		}
	case netlink.LinkNotFoundError:
		return fmt.Errorf("bridge does not exist: %s", attrs.Ifname)
	default:
		return fmt.Errorf("netlink: %s", err)
	}

	link, err := netlink.LinkByName(linkname)
	if err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	if err := netlink.LinkSetMaster(link, brLink); err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	if attrs.MTU >= 68 {
		if err := netlink.LinkSetMTU(link, int(attrs.MTU)); err != nil {
			return fmt.Errorf("netlink: %s: %s", link.Attrs().Name, err)
		}
	}

	return nil
}

func DeconfigureBridgePort(linkname string, brname string) error {
	link, err := netlink.LinkByName(linkname)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			// link already removed, so do nothing
			return nil
		}
		return fmt.Errorf("netlink: %s", err)
	}

	if err := netlink.LinkSetNoMaster(link); err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	return nil
}
