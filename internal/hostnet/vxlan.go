package hostnet

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

type VxlanDeviceAttrs struct {
	VNI   uint32
	MTU   uint32
	Local net.IP
}

func ConfigureVxlanPort(linkname string, attrs *VxlanDeviceAttrs, secondStage bool) error {
	if secondStage {
		// no second stage for this scheme
		return nil
	}

	vxName := fmt.Sprintf("vxlan_%d", attrs.VNI)
	brName := fmt.Sprintf("xbr_%d", attrs.VNI)

	brLink, err := CreateBridgeIfNotExist(brName)
	if err != nil {
		return err
	}

	vxLink, err := CreateVxlanIfNotExist(vxName, attrs.VNI, attrs.Local)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetMaster(vxLink, brLink); err != nil {
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
		for _, l := range []netlink.Link{brLink, vxLink, link} {
			if err := netlink.LinkSetMTU(l, int(attrs.MTU)); err != nil {
				return fmt.Errorf("netlink: %s: %s", l.Attrs().Name, err)
			}
		}
	}

	return nil
}

func DeconfigureVxlanPort(linkname string, vni uint32) error {
	if err := RemoveLinkIfExist(linkname); err != nil {
		return err
	}

	vxName := fmt.Sprintf("vxlan_%d", vni)
	brName := fmt.Sprintf("xbr_%d", vni)

	brLink, err := netlink.LinkByName(brName)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return RemoveLinkIfExist(vxName)
		}
		return err
	}
	brIndex := brLink.Attrs().Index

	links, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	for _, link := range links {
		attrs := link.Attrs()
		if attrs.MasterIndex == brIndex && attrs.Name != vxName {
			// There are some other devices in the bridge
			return nil
		}
	}

	// No other devices in the bridge. Remove it and its vxlan device
	if err := RemoveLinkIfExist(vxName); err != nil {
		return err
	}

	if err := netlink.LinkDel(brLink); err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	return nil
}

func CreateVxlanIfNotExist(linkname string, vni uint32, srcIP net.IP) (netlink.Link, error) {
	var link netlink.Link
	var err error

	link, err = netlink.LinkByName(linkname)
	switch err.(type) {
	case nil:
		if v, ok := link.(*netlink.Vxlan); ok {
			if uint32(v.VxlanId) != vni {
				return nil, fmt.Errorf("vxlan device already exists, but with a different vni (%d): %s", v.VxlanId, linkname)
			}
		} else {
			return nil, fmt.Errorf("device already exists but is not a vxlan device: %s", linkname)
		}
	case netlink.LinkNotFoundError:
		attrs := netlink.NewLinkAttrs()
		attrs.Name = linkname

		link = &netlink.Vxlan{
			LinkAttrs: attrs,
			VxlanId:   int(vni),
			SrcAddr:   srcIP,
			Port:      4789,
		}

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
