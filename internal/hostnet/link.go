package hostnet

import (
	"crypto/md5"
	"fmt"
	"math/big"

	"github.com/vishvananda/netlink"
)

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

func GetLinkID(linkname string, linkindex int) uint16 {
	h := md5.New()

	fmt.Fprintf(h, "%s:%d", linkname, linkindex)

	bi := big.NewInt(0)
	bi.SetBytes(h.Sum(nil))

	// Should be a number between 200 and 65000
	x := big.NewInt(0).Mod(bi, big.NewInt(64800)).Int64() + 200

	return uint16(x)
}
