package hostnet

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/0xef53/kvmrun/kvmrun"

	"github.com/vishvananda/netlink"
)

func RemoveLinkIfExist(linkname string) error {
	link, err := netlink.LinkByName(linkname)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return nil
		}
		return fmt.Errorf("netlink: %w", err)
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

type LinkID uint16

func GetLinkID(linkname string, linkindex int) LinkID {
	h := md5.New()

	fmt.Fprintf(h, "%s:%d", linkname, linkindex)

	bi := big.NewInt(0)
	bi.SetBytes(h.Sum(nil))

	// Should be a number between 200 (0xc8) and 52000 (0xcb20)
	x := big.NewInt(0).Mod(bi, big.NewInt(51800)).Int64() + 200

	return LinkID(x)
}

func (v LinkID) ClassID() uint32 {
	return netlink.MakeHandle(1, uint16(v))
}

func (v LinkID) String() string {
	return fmt.Sprintf("1:0x%x", uint16(v))
}

func ensureLink(link netlink.Link) {
	if link == nil {
		panic("link is not specified")
	}
}

var ErrLinkNotFound = errors.New("link not found")

func LinkFromDumpFile(linkname string) (netlink.Link, error) {
	link := netlink.GenericLink{
		LinkAttrs: netlink.NewLinkAttrs(),
		LinkType:  "generic",
	}

	b, err := os.ReadFile(filepath.Join(kvmrun.NETWORKDIR, linkname+".link"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %w", ErrLinkNotFound, err)
		}

		return nil, err
	}

	if err := json.Unmarshal(b, &link.LinkAttrs); err != nil {
		return nil, err
	}

	return &link, nil
}

func LinkWriteDumpFile(link netlink.Link) error {
	if err := os.MkdirAll(kvmrun.NETWORKDIR, 0755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(link.Attrs(), "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(kvmrun.NETWORKDIR, link.Attrs().Name+".link"), b, 0644)
}
