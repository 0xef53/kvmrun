package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"strings"

	cg "github.com/0xef53/kvmrun/internal/cgroups"
	"github.com/0xef53/kvmrun/internal/garp"
	"github.com/0xef53/kvmrun/internal/helpers"
	"github.com/0xef53/kvmrun/internal/ipmath"
	"github.com/0xef53/kvmrun/internal/mapdb"

	"github.com/vishvananda/netlink"
)

var (
	linkDB = mapdb.New("/var/run/kvm-network/linkdb")

	ErrCgroupBinding = errors.New("failed to configure cgroup")
)

type RouterDeviceAttrs struct {
	Addrs          []string
	MTU            uint32
	BindInterface  string
	DefaultGateway string
	InLimit        uint32
	OutLimit       uint32
	ProcessID      uint32
}

func ConfigureRouter(linkname string, attrs *RouterDeviceAttrs, secondStage bool) error {
	if secondStage {
		return ConfigureRouterAddrs(linkname, attrs)
	}

	return ConfigureRouterInterface(linkname, attrs)
}

func ConfigureRouterInterface(linkname string, attrs *RouterDeviceAttrs) error {
	if attrs.OutLimit > 0 && len(attrs.BindInterface) == 0 {
		return fmt.Errorf("can not setup outbound limit: bind_interface is not set")
	}

	link, err := netlink.LinkByName(linkname)
	if err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	linkID, err := linkDB.Get(linkname)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	if attrs.MTU >= 68 {
		if err := netlink.LinkSetMTU(link, int(attrs.MTU)); err != nil {
			return fmt.Errorf("netlink: %s: %s", linkname, err)
		}
	}

	if err := createBlackholeRules(linkname); err != nil {
		return err
	}

	if err := setInboundLimits(linkname, attrs.InLimit); err != nil {
		return err
	}

	if len(attrs.BindInterface) > 0 {
		if err := setOutboundLimits(linkname, linkID, attrs.ProcessID, attrs.OutLimit, attrs.BindInterface); err != nil {
			return err
		}
	}

	return nil
}

func createBlackholeRules(linkname string) error {
	/*
		TODO: should be rewritten using the "netlink" library

		See https://github.com/vishvananda/netlink/issues/838 for details
	*/

	for _, f := range []string{"inet", "inet6"} {
		args := []string{"-family", f, "rule", "add", "iif", linkname, "blackhole"}

		if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create blackhole rule for %s (family = %s): %s", linkname, f, strings.TrimSpace(string(out)))
		}
	}

	return nil
}

func setInboundLimits(linkname string, rate uint32) error {
	/*
		TODO: should be rewritten using the "netlink" library
	*/

	// We don't care about possible errors
	exec.Command("tc", "qdisc", "del", "dev", linkname, "root").Run()

	if rate == 0 {
		return nil
	}

	// Make new configuration
	qdiscArgs := []string{"qdisc", "replace", "dev", linkname, "root", "handle", "1", "htb", "default", "1"}

	if out, err := exec.Command("tc", qdiscArgs...).CombinedOutput(); err != nil {
		if exitCode, ok := helpers.CommandExitCode(err); !ok || exitCode != 2 {
			return fmt.Errorf("failed to create qdisc rule for %s (%s): %s", linkname, err, strings.TrimSpace(string(out)))
		}
	}

	classArgs := []string{"class", "replace", "dev", linkname, "parent", "1:", "classid", "1:1", "htb", "rate", fmt.Sprintf("%dmbit", rate)}

	if out, err := exec.Command("tc", classArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create class rule for %s (%s): %s", linkname, err, strings.TrimSpace(string(out)))
	}

	return nil
}

func setOutboundLimits(linkname string, linkID int, pid, rate uint32, bindInterface string) error {
	/*
		TODO: should be rewritten using the "netlink" library
	*/

	// We don't care about possible errors
	exec.Command("tc", "class", "del", "dev", bindInterface, "classid", fmt.Sprintf("1:0x%x", linkID)).Run()

	if rate == 0 {
		return nil
	}

	// Make new configuration
	qdiscArgs := []string{"qdisc", "add", "dev", bindInterface, "root", "handle", "1", "htb"}

	// Try to add htb discipline to the root of bindInterface.
	// If discipline is exist, the return code will be 2.
	if out, err := exec.Command("tc", qdiscArgs...).CombinedOutput(); err != nil {
		if exitCode, ok := helpers.CommandExitCode(err); !ok || exitCode != 2 {
			return fmt.Errorf("failed to create qdisc rule for %s (%s): %s", bindInterface, err, strings.TrimSpace(string(out)))
		}
	}

	filterArgs := []string{"filter", "replace", "dev", bindInterface, "parent", "1:", "protocol", "all", "prio", "10", "handle", "1:", "cgroup"}

	if out, err := exec.Command("tc", filterArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create filter rule for %s (%s): %s", bindInterface, err, strings.TrimSpace(string(out)))
	}

	classArgs := []string{"class", "add", "dev", bindInterface, "parent", "1:", "classid", fmt.Sprintf("1:0x%x", linkID), "htb", "rate", fmt.Sprintf("%dmbit", rate)}

	if out, err := exec.Command("tc", classArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create class rule for %s (linkname = %s, classid = 1:0x%x) (%s): %s", bindInterface, linkname, linkID, err, strings.TrimSpace(string(out)))
	}

	// Try to set net_cls.classid for the virt.machine process
	err := func() error {
		if pid > 0 {
			cgmgr, err := cg.LoadManager(int(pid))
			if err != nil {
				return err
			}

			if err := cgmgr.SetNetClassID(int64(65536 + linkID)); err != nil {
				return err
			}
		}

		return nil
	}()

	if err != nil {
		return fmt.Errorf("%w: %s", ErrCgroupBinding, err)
	}

	return nil
}

func ConfigureRouterAddrs(linkname string, attrs *RouterDeviceAttrs) error {
	link, err := netlink.LinkByName(linkname)
	if err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	for _, addr := range attrs.Addrs {
		if err := addRoute(link, addr, "main"); err != nil {
			return err
		}
		if err := addRule(link, addr, "main"); err != nil {
			return err
		}
	}

	// Send Gratuitous ARP for all router gateways
	if err := announceRouterGateways(link, attrs); err != nil {
		return err
	}

	return nil
}

func addRoute(link netlink.Link, addr, table string) error {
	tableNum, err := GetRouteTableIndex(table)
	if err != nil {
		return err
	}

	ip, err := ParseIPNet(addr)
	if err != nil {
		return err
	}

	maskOnes, maskBits := ip.Mask.Size()

	if ip.IP.To4() != nil {
		if maskOnes <= 30 {
			lastIP, _ := ipmath.GetLastIPv4(ip)

			gwAddr := netlink.Addr{
				IPNet: &net.IPNet{
					IP:   net.ParseIP(netip.MustParseAddr(lastIP.String()).Prev().String()),
					Mask: net.CIDRMask(maskOnes, maskBits),
				},
			}

			if err := netlink.AddrAdd(link, &gwAddr); err != nil {
				return err
			}
		}
	} else {
		if maskOnes <= 64 {
			lastIP, _ := ipmath.GetLastIPv6(ip)

			lastAddr := netlink.Addr{
				IPNet: &net.IPNet{
					IP:   lastIP,
					Mask: net.CIDRMask(maskOnes, maskBits),
				},
			}

			if err := netlink.AddrAdd(link, &lastAddr); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("too small ipv6 netmask")
		}
	}

	_, dst, err := net.ParseCIDR(addr)
	if err != nil {
		return err
	}

	r := netlink.Route{
		LinkIndex: link.Attrs().Index,
		Scope:     netlink.SCOPE_LINK,
		Table:     tableNum,
		Dst:       dst,
	}

	if err := netlink.RouteReplace(&r); err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	return nil
}

func addRule(link netlink.Link, addr, table string) error {
	tableNum, err := GetRouteTableIndex(table)
	if err != nil {
		return err
	}

	ip, err := ParseIPNet(addr)
	if err != nil {
		return err
	}

	rule := netlink.NewRule()

	rule.Table = tableNum
	rule.IifName = link.Attrs().Name
	rule.Src = ip

	if ip.IP.To4() != nil {
		rule.Family = netlink.FAMILY_V4
	} else {
		rule.Family = netlink.FAMILY_V6
	}

	if err := netlink.RuleAdd(rule); err != nil {
		return err
	}

	return nil
}

func announceRouterGateways(link netlink.Link, attrs *RouterDeviceAttrs) error {
	gws := make(map[string]struct{})

	for _, addr := range attrs.Addrs {
		ip, err := ParseIPNet(addr)
		if err != nil {
			return err
		}

		if ip.IP.To4() == nil {
			// Only IPv4 addrs are supported
			continue
		}

		var gw string

		maskOnes, _ := ip.Mask.Size()

		if maskOnes <= 30 {
			lastIP, _ := ipmath.GetLastIPv4(ip)

			gw = netip.MustParseAddr(lastIP.String()).Prev().String()
		} else {
			gw = attrs.DefaultGateway
		}

		if _, ok := gws[gw]; !ok && len(gw) > 0 {
			gws[gw] = struct{}{}

			go garp.Send(context.Background(), link.Attrs().Name, gw, 10, 1)
		}
	}

	return nil
}

func DeconfigureRouter(linkname, bindInterface string) error {
	// Remove all rules including blackhole
	if rules, err := netlink.RuleList(netlink.FAMILY_ALL); err == nil {
		for _, rule := range rules {
			if rule.IifName == linkname {
				netlink.RuleDel(&rule)
			}
		}
	}

	removeBlackholeRulesV6(linkname)

	// Remove all routes and addresses
	if link, err := netlink.LinkByName(linkname); err == nil {
		if addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL); err == nil {
			for _, addr := range addrs {
				if addr.IP.IsLinkLocalUnicast() || addr.IP.IsLinkLocalMulticast() {
					continue
				}
				netlink.AddrDel(link, &addr)
			}
		}
		if routes, err := netlink.RouteList(link, netlink.FAMILY_ALL); err == nil {
			for _, route := range routes {
				if route.Dst.IP.IsLinkLocalUnicast() || route.Dst.IP.IsLinkLocalMulticast() {
					continue
				}
				netlink.RouteDel(&route)
			}
		}
	}

	// Remove all TC rules
	setInboundLimits(linkname, 0)

	if len(bindInterface) > 0 {
		if linkID, err := linkDB.Delete(linkname); err == nil && linkID != -1 {
			setOutboundLimits(linkname, linkID, 0, 0, bindInterface)
		}
	}

	return nil
}

func removeBlackholeRulesV6(linkname string) error {
	/*
		TODO: should be rewritten using the "netlink" library

		See https://github.com/vishvananda/netlink/issues/838 for details
	*/

	args := []string{"-family", "inet6", "rule", "del", "from", "all", "iif", linkname, "blackhole"}

	for {
		if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
			if exitCode, ok := helpers.CommandExitCode(err); ok && exitCode == 2 {
				break
			}
			return fmt.Errorf("failed to remove IPv6 blackhole rule for %s (%s): %s", linkname, err, strings.TrimSpace(string(out)))
		}
	}

	return nil
}
