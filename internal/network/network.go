package network

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

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

func RemoveLinkIfExist(linkname string) error {
	link, err := netlink.LinkByName(linkname)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return nil
		}
		return fmt.Errorf("netlink: %s", err)
	}

	switch link.(type) {
	case *netlink.Vlan, *netlink.Vxlan, *netlink.Tuntap:
	default:
		return fmt.Errorf("unsupported device type: %s", linkname)
	}

	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	return nil
}
