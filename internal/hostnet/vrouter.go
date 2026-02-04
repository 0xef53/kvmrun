package hostnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	cg "github.com/0xef53/kvmrun/internal/cgroups"
	"github.com/0xef53/kvmrun/internal/garp"
	"github.com/0xef53/kvmrun/internal/ipmath"
	"github.com/0xef53/kvmrun/internal/utils"

	"github.com/vishvananda/netlink"
)

var (
	ErrCgroupBinding = errors.New("failed to configure cgroup")
)

type VirtualRouterAttrs struct {
	Addrs         []string
	MTU           uint32
	BindInterface string
	Gateway4      string
	Gateway6      string
	InLimit       uint32
	OutLimit      uint32

	ProcessID uint32
}

func RouterConfigure(linkname string, attrs *VirtualRouterAttrs, secondStage bool) error {
	if secondStage {
		return RouterConfigureAddrs(linkname, attrs.Addrs, attrs.Gateway4, attrs.Gateway6)
	}

	return RouterConfigureInterface(linkname, attrs)
}

func RouterConfigureInterface(linkname string, attrs *VirtualRouterAttrs) error {
	if attrs.OutLimit > 0 && len(attrs.BindInterface) == 0 {
		return fmt.Errorf("can not setup outbound limit: bind_interface is not set")
	}

	link, err := netlink.LinkByName(linkname)
	if err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	linkID := GetLinkID(linkname, link.Attrs().Index)

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	if attrs.MTU >= 68 {
		if err := netlink.LinkSetMTU(link, int(attrs.MTU)); err != nil {
			return fmt.Errorf("netlink: %s: %s", linkname, err)
		}
	}

	if err := routerCreateBlackholeRules(linkname); err != nil {
		return err
	}

	if err := routerSetInboundLimits(linkname, attrs.InLimit); err != nil {
		return err
	}

	if len(attrs.BindInterface) > 0 {
		if err := routerSetOutboundLimits(linkname, linkID, attrs.ProcessID, attrs.OutLimit, attrs.BindInterface); err != nil {
			return err
		}

		if b, err := json.MarshalIndent(link.Attrs(), "", "    "); err == nil {
			if err := os.WriteFile(filepath.Join("/run/kvm-network", linkname), b, 0644); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func routerCreateBlackholeRules(linkname string) error {
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

func RouterSetInboundLimits(linkname string, rate uint32) error {
	return routerSetInboundLimits(linkname, rate)
}

func routerSetInboundLimits(linkname string, rate uint32) error {
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
		if exitCode, ok := utils.CommandExitCode(err); !ok || exitCode != 2 {
			return fmt.Errorf("failed to create qdisc rule for %s (%s): %s", linkname, err, strings.TrimSpace(string(out)))
		}
	}

	classArgs := []string{"class", "replace", "dev", linkname, "parent", "1:", "classid", "1:1", "htb", "rate", fmt.Sprintf("%dmbit", rate)}

	if out, err := exec.Command("tc", classArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create class rule for %s (%s): %s", linkname, err, strings.TrimSpace(string(out)))
	}

	return nil
}

func RouterSetOutboundLimits(linkname string, rate uint32, bindInterface string, pid uint32) error {
	link, err := netlink.LinkByName(linkname)
	if err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

	linkID := GetLinkID(linkname, link.Attrs().Index)

	if _, err := netlink.LinkByName(bindInterface); err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

	return routerSetOutboundLimits(linkname, linkID, pid, rate, bindInterface)
}

func routerSetOutboundLimits(linkname string, linkID uint16, pid, rate uint32, bindInterface string) error {
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
		if exitCode, ok := utils.CommandExitCode(err); !ok || exitCode != 2 {
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

			if err := cgmgr.SetNetClassID(int64(65536 + int(linkID))); err != nil {
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

func RouterConfigureAddrs(linkname string, addrs []string, gateway4, gateway6 string) error {
	link, err := netlink.LinkByName(linkname)
	if err != nil {
		return fmt.Errorf("netlink: %s", err)
	}

	for _, addr := range addrs {
		if err := routerAddRoute(link, addr, "main"); err != nil {
			fmt.Printf("DEBUG ConfigureRouterAddrs(): addRoute err (type = %T): %+v\n", err, err)
			return err
		}
		if err := routerAddRule(link, addr, "main"); err != nil {
			fmt.Printf("DEBUG ConfigureRouterAddrs(): addRule err (type = %T): %+v\n", err, err)
			return err
		}
	}

	// Send Gratuitous ARP for all router gateways
	if err := routerAnnounceGateways(link, addrs, gateway4); err != nil {
		return err
	}

	return nil
}

func routerAddRoute(link netlink.Link, addr, table string) error {
	tableNum, err := utils.GetRouteTableIndex(table)
	if err != nil {
		return err
	}

	ip, err := utils.ParseIPNet(addr)
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

func routerAddRule(link netlink.Link, addr, table string) error {
	tableNum, err := utils.GetRouteTableIndex(table)
	if err != nil {
		return err
	}

	ip, err := utils.ParseIPNet(addr)
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

func routerAnnounceGateways(link netlink.Link, addrs []string, gateway4 string) error {
	gws := make(map[string]struct{})

	for _, addr := range addrs {
		ip, err := utils.ParseIPNet(addr)
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
			gw = gateway4
		}

		if _, ok := gws[gw]; !ok && len(gw) > 0 {
			gws[gw] = struct{}{}

			go garp.Send(context.Background(), link.Attrs().Name, gw, 10, 1)
		}
	}

	return nil
}

func RouterDeconfigure(linkname, bindInterface string) error {
	// Remove all rules including blackhole
	if rules, err := netlink.RuleList(netlink.FAMILY_ALL); err == nil {
		for _, rule := range rules {
			if rule.IifName == linkname {
				netlink.RuleDel(&rule)
			}
		}
	}

	routerRemoveBlackholeRulesV6(linkname)

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
	routerSetInboundLimits(linkname, 0)

	if len(bindInterface) > 0 {
		attrs := struct {
			Index int `json:"index"`
		}{}

		if b, err := os.ReadFile(filepath.Join("/run/kvm-network", linkname)); err == nil {
			if err := json.Unmarshal(b, &attrs); err != nil {
				return err
			}
		} else {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		linkID := GetLinkID(linkname, attrs.Index)

		routerSetOutboundLimits(linkname, linkID, 0, 0, bindInterface)

		os.Remove(filepath.Join("/run/kvm-network", linkname))
	}

	return nil
}

func routerRemoveBlackholeRulesV6(linkname string) error {
	/*
		TODO: should be rewritten using the "netlink" library

		See https://github.com/vishvananda/netlink/issues/838 for details
	*/

	args := []string{"-family", "inet6", "rule", "del", "from", "all", "iif", linkname, "blackhole"}

	for {
		if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
			if exitCode, ok := utils.CommandExitCode(err); ok && exitCode == 2 {
				break
			}
			return fmt.Errorf("failed to remove IPv6 blackhole rule for %s (%s): %s", linkname, err, strings.TrimSpace(string(out)))
		}
	}

	return nil
}
