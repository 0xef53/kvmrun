package hostnet

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

type BridgePortAttrs struct {
	BridgeName string
	MTU        uint32
}

func BridgePortConfigure(linkname string, attrs *BridgePortAttrs, secondStage bool) error {
	if secondStage {
		// no second stage for this scheme
		return nil
	}

	brLink, err := netlink.LinkByName(attrs.BridgeName)

	switch err.(type) {
	case nil:
		if _, ok := brLink.(*netlink.Bridge); !ok {
			return fmt.Errorf("not a bridge device: %s", attrs.BridgeName)
		}
	case netlink.LinkNotFoundError:
		return fmt.Errorf("bridge does not exist: %s", attrs.BridgeName)
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

func BridgePortDeconfigure(linkname string, brname string) error {
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

func CreateBridgeIfNotExist(linkname string) (netlink.Link, error) {
	var link netlink.Link
	var err error

	link, err = netlink.LinkByName(linkname)

	switch err.(type) {
	case nil:
		if _, ok := link.(*netlink.Bridge); !ok {
			return nil, fmt.Errorf("device already exists but is not a bridge: %s", linkname)
		}
	case netlink.LinkNotFoundError:
		attrs := netlink.NewLinkAttrs()
		attrs.Name = linkname

		link = &netlink.Bridge{LinkAttrs: attrs}

		if err := netlink.LinkAdd(link); err != nil {
			return nil, fmt.Errorf("netlink: %s", err)
		}
	default:
		return nil, fmt.Errorf("netlink: %s", err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return nil, fmt.Errorf("netlink: %s", err)
	}

	return link, nil
}
