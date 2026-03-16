package hostnet

/*
	TODO: probably all the TC functions should be rewritten using the "netlink" library
*/

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/0xef53/kvmrun/internal/utils"
	"golang.org/x/sys/unix"

	"github.com/vishvananda/netlink"
)

func tcCreateQdisc(bindIface string) error {
	args := []string{
		"qdisc",
		"replace",
		"dev", bindIface,
		"root",
		"handle", "1",
		"htb",
		"default", "1",
	}

	// Try to add htb discipline to the root of bindIface.
	// If discipline is exist, the return code will be 2.
	out, err := exec.Command("tc", args...).CombinedOutput()
	if err != nil {
		if exitCode, ok := utils.CommandExitCode(err); !ok || exitCode != 2 {
			return fmt.Errorf(
				"failed to create qdisc on %s (%w): %s",
				bindIface,
				err,
				strings.TrimSpace(string(out)),
			)
		}
	}

	return nil
}

func tcRemoveQdisc(bindIface string) error {
	args := []string{
		"qdisc",
		"del",
		"dev", bindIface,
		"root",
	}

	out, err := exec.Command("tc", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"failed to remove qdisc on %s (%w): %s",
			bindIface,
			err,
			strings.TrimSpace(string(out)),
		)
	}

	return nil
}

func tcCreateClass(bindIface string, classID, rateMbit uint32) error {
	major, minor := netlink.MajorMinor(classID)

	strClassID := fmt.Sprintf("%x:%x", major, minor)

	args := []string{
		"class",
		"replace",
		"dev", bindIface,
		"parent", "1:",
		"classid", strClassID,
		"htb",
		"rate", fmt.Sprintf("%dmbit", rateMbit),
	}

	out, err := exec.Command("tc", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"failed to create class on %s (class_id = %s) (%w): %s",
			bindIface,
			strClassID,
			err,
			strings.TrimSpace(string(out)),
		)
	}

	return nil
}

func tcRemoveClass(bindIface string, classID uint32) error {
	major, minor := netlink.MajorMinor(classID)

	strClassID := fmt.Sprintf("%x:%x", major, minor)

	args := []string{
		"class",
		"del",
		"dev", bindIface,
		"classid", strClassID,
	}

	if out, err := exec.Command("tc", args...).CombinedOutput(); err != nil {
		return fmt.Errorf(
			"failed to delete class on %s (class_id = %s) (%s): %s",
			bindIface,
			strClassID,
			err,
			strings.TrimSpace(string(out)),
		)
	}

	return nil
}

type AddrDirection uint8

const (
	ADDR_DIRECTION_SRC AddrDirection = iota
	ADDR_DIRECTION_DST
)

func (d AddrDirection) String() string {
	switch d {
	case ADDR_DIRECTION_SRC:
		return "src"
	case ADDR_DIRECTION_DST:
		return "dst"
	}

	return "UNKNOWN"
}

func tcAddFilter(bindIface string, classID uint32, prefix string, direction AddrDirection) error {
	ipnet, err := utils.ParseIPNet(prefix)
	if err != nil {
		return err
	}

	var proto, selProto string

	if ipnet.IP.To4() != nil {
		proto = "ip"
		selProto = "ip"
	} else {
		proto = "ipv6"
		selProto = "ip6"
	}

	flowID := fmt.Sprintf("%d:0x%x", int(classID/65536), classID%65536)

	args := []string{
		"filter",
		"add",
		"dev", bindIface,
		"parent", "1:",
		"protocol", proto,
		"u32",
		"match", selProto, direction.String(), ipnet.String(), "flowid", flowID,
	}

	if out, err := exec.Command("tc", args...).CombinedOutput(); err != nil {
		return fmt.Errorf(
			"failed to create filter on %s (addr = %s, class_id = %s) (%s): %s",
			bindIface,
			ipnet.String(),
			flowID,
			err,
			strings.TrimSpace(string(out)),
		)
	}

	return nil
}

func tcRemoveFilters(bindIface string, classID uint32, prefixes ...string) error {
	if classID == 0 {
		return nil
	}

	bindLink, err := netlink.LinkByName(bindIface)
	if err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	filters, err := netlink.FilterList(bindLink, 0)
	if err != nil {
		return fmt.Errorf("filter list failed: %w", err)
	}

	removeByPrio := func(prio uint16) error {
		strClassID := fmt.Sprintf("%d:%x", int(classID/65536), classID%65536)

		args := []string{
			"filter",
			"del",
			"dev", bindIface,
			"pref", fmt.Sprintf("%d", prio),
		}

		if out, err := exec.Command("tc", args...).CombinedOutput(); err != nil {
			return fmt.Errorf(
				"failed to remove filter on %s (prio = %d, class_id = %s) (%w): %s",
				bindIface,
				prio,
				strClassID,
				err,
				strings.TrimSpace(string(out)),
			)
		}

		return nil
	}

	candidates := make(map[uint16]netlink.Filter)

	for _, filter := range filters {
		// We are only interested in u32 filters with specified classID ...
		if v, ok := filter.(*netlink.U32); ok && v.Sel != nil && v.ClassId == classID {
			// ... and IPv4/IPv6 addresses in selectors
			if v.Protocol == unix.ETH_P_IP || v.Protocol == unix.ETH_P_IPV6 {
				candidates[v.Priority] = filter
			}
		}
	}

	if len(prefixes) > 0 { // remove filters only for the specified prefixes
		normalized := make(map[string]struct{})

		for _, s := range prefixes {
			_, ipnet, err := utils.ParseCIDR(s)
			if err != nil {
				return err
			}

			normalized[ipnet.String()] = struct{}{}
		}

		for _, filter := range candidates {
			u32 := filter.(*netlink.U32)

			info, err := tcParseU32Sel(u32.Protocol, u32.Sel)
			if err != nil {
				return err
			}

			if _, ok := normalized[info.Addr.String()]; ok {
				if err := removeByPrio(u32.Priority); err != nil {
					return err
				}
			}
		}
	} else { // remove all filters with this class ID
		for _, filter := range candidates {
			u32 := filter.(*netlink.U32)

			if err := removeByPrio(u32.Priority); err != nil {
				return err
			}
		}
	}

	return nil
}

type U32Info struct {
	Addr     net.IPNet
	AddrType AddrDirection
}

func tcParseU32Sel(protocol uint16, sel *netlink.TcU32Sel) (*U32Info, error) {
	if count := len(sel.Keys); !(count == 1 || count == 2) {
		return nil, fmt.Errorf("unsupported length of u32 selector keys")
	}

	info := U32Info{}

	switch protocol {
	case unix.ETH_P_IP:
		switch sel.Keys[0].Off {
		case 12:
			info.AddrType = ADDR_DIRECTION_SRC
		case 16:
			info.AddrType = ADDR_DIRECTION_DST
		default:
			return nil, fmt.Errorf("unknown IPv4 key offset: %d", sel.Keys[0].Off)
		}

		info.Addr = net.IPNet{
			IP:   utils.IntToIPv4(sel.Keys[0].Val),
			Mask: net.IPMask(utils.IntToIPv4(sel.Keys[0].Mask)),
		}

		return &info, nil
	case unix.ETH_P_IPV6:
		if len(sel.Keys) < 2 {
			return nil, fmt.Errorf("insufficient keys for IPv6 address")
		}

		switch sel.Keys[0].Off {
		case 8:
			info.AddrType = ADDR_DIRECTION_SRC
		case 24:
			info.AddrType = ADDR_DIRECTION_DST
		default:
			return nil, fmt.Errorf("unknown IPv6 key offset: %d", sel.Keys[0].Off)
		}

		info.Addr = net.IPNet{
			IP:   make(net.IP, net.IPv6len),
			Mask: make(net.IPMask, net.IPv6len),
		}

		// Each key contains 4 bytes, put them to the IP
		for idx := 0; idx < 2; idx++ {
			val := sel.Keys[idx].Val

			info.Addr.IP[idx*4] = byte(val >> 24)
			info.Addr.IP[idx*4+1] = byte(val >> 16)
			info.Addr.IP[idx*4+2] = byte(val >> 8)
			info.Addr.IP[idx*4+3] = byte(val)
		}

		// Do the same with the mask
		for idx := 0; idx < 2; idx++ {
			val := sel.Keys[idx].Mask

			info.Addr.Mask[idx*4] = byte(val >> 24)
			info.Addr.Mask[idx*4+1] = byte(val >> 16)
			info.Addr.Mask[idx*4+2] = byte(val >> 8)
			info.Addr.Mask[idx*4+3] = byte(val)
		}

		return &info, nil
	}

	return nil, fmt.Errorf("unsupported protocol of filter")
}
