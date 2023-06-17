package network

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"math/big"
	"net"
	"os"
	"strconv"
	"strings"

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

func GetLinkID(linkname string, linkindex int) uint16 {
	h := md5.New()
	h.Write([]byte(fmt.Sprintf("%s:%d", linkname, linkindex)))

	bi := big.NewInt(0)
	bi.SetBytes(h.Sum(nil))

	// Should be a number between 200 and 65000
	x := big.NewInt(0).Mod(bi, big.NewInt(64800)).Int64() + 200

	return uint16(x)
}

func ParseIPNet(s string) (*net.IPNet, error) {
	if !strings.Contains(s, "/") {
		if net.ParseIP(s).To4() != nil {
			s += "/32"
		} else {
			s += "/128"
		}
	}

	return netlink.ParseIPNet(s)
}

func GetRouteTableIndex(table string) (int, error) {
	fd, err := os.Open("/etc/iproute2/rt_tables")
	if err != nil {
		return -1, err
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}

		ff := strings.Fields(line)

		if len(ff) == 2 && strings.ToLower(ff[1]) == table {
			if v, err := strconv.Atoi(ff[0]); err == nil {
				return v, nil
			} else {
				return -1, err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return -1, err
	}

	return -1, fmt.Errorf("table not found: %s", table)
}
