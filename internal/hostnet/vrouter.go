package hostnet

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/0xef53/kvmrun/internal/garp"
	"github.com/0xef53/kvmrun/internal/ipmath"
	"github.com/0xef53/kvmrun/internal/utils"
	"github.com/0xef53/kvmrun/kvmrun"

	"github.com/vishvananda/netlink"
)

type VirtualRouterAttrs struct {
	Addrs          []string
	UnmanagedAddrs []string
	MTU            uint32
	BindIface      string
	Gateway4       string
	Gateway6       string
	InLimit        uint32
	OutLimit       uint32
}

func RouterConfigure(linkname string, attrs *VirtualRouterAttrs, secondStage bool) error {
	if secondStage {
		if err := RouterConfigureAddrs(attrs.BindIface, linkname, false, attrs.Addrs...); err != nil {
			return nil
		}

		if err := RouterConfigureAddrs(attrs.BindIface, linkname, true, attrs.UnmanagedAddrs...); err != nil {
			return nil
		}

		// Send Gratuitous ARP for all router gateways
		return RouterAnnounceGateways(linkname, append(attrs.Addrs, attrs.UnmanagedAddrs...), attrs.Gateway4)
	}

	return RouterConfigureInterface(linkname, attrs)
}

func RouterDeconfigure(linkname, tcBindIface string) error {
	// Read the attributes from a saved dump file, since we cannot know for sure
	// that the physical link still exists (custom ifup/ifdown scripts may have
	// already deleted it).
	// If no dump file, ErrLinkNotFound will be returned.
	link, err := LinkFromDumpFile(linkname)
	if err != nil {
		return err
	}
	defer os.Remove(filepath.Join(kvmrun.NETWORKDIR, linkname+".link"))

	// Remove all rules including IPv4/IPv6 blackhole
	routerRemoveRules(link)

	// Remove all routes and GW addresses
	routerRemoveRoutes(link)

	// Remove QoS configuration for incoming traffic
	routerSetInboundLimits(link, 0)

	// Remove QoS configuration for outgoing traffic
	if len(tcBindIface) > 0 {
		routerSetOutboundLimits(GetLinkID(linkname, link.Attrs().Index), 0, tcBindIface)
	}

	return nil
}

func RouterConfigureInterface(linkname string, attrs *VirtualRouterAttrs) error {
	if attrs.OutLimit > 0 && len(attrs.BindIface) == 0 {
		return fmt.Errorf("can not setup outbound limit: bind_interface is not set")
	}

	link, err := netlink.LinkByName(linkname)
	if err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

	// Save the dump with the network interface attributes.
	// It will be needed in the RouterDeconfigure() function,
	// which may no longer exist by the time it's called.
	if err := LinkWriteDumpFile(link); err != nil {
		return fmt.Errorf("cannot write link dump file: %w", err)
	}

	linkID := GetLinkID(linkname, link.Attrs().Index)

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

	if attrs.MTU >= 68 {
		if err := netlink.LinkSetMTU(link, int(attrs.MTU)); err != nil {
			return fmt.Errorf("netlink: %s: %w", linkname, err)
		}
	}

	// IPv4 && IPv6 blackhole rules
	if err := routerAddBlackholeRules(link); err != nil {
		return err
	}

	if err := routerSetInboundLimits(link, attrs.InLimit); err != nil {
		return err
	}

	if len(attrs.BindIface) > 0 {
		if err := routerSetOutboundLimits(linkID, attrs.OutLimit, attrs.BindIface); err != nil {
			return err
		}
	}

	return nil
}

func RouterConfigureAddrs(tcBindIface, linkname string, unmanaged bool, addrs ...string) error {
	link, err := netlink.LinkByName(linkname)
	if err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

	linkID := GetLinkID(linkname, link.Attrs().Index)

	for _, addr := range addrs {
		if err := routerAddRoute(link, addr, "main"); err != nil {
			return fmt.Errorf("failed to create route: %w", err)
		}

		if err := routerAddRule(link, addr, "main"); err != nil {
			return fmt.Errorf("failed to create rule: %w", err)
		}

		if len(tcBindIface) > 0 && !unmanaged {
			if err := tcAddFilter(tcBindIface, linkID.ClassID(), addr, ADDR_DIRECTION_SRC); err != nil {
				return err
			}
		}
	}

	return nil
}

func RouterDeconfigureAddrs(tcBindIface, linkname string, unmanaged bool, addrs ...string) error {
	link, err := netlink.LinkByName(linkname)
	if err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

	linkID := GetLinkID(linkname, link.Attrs().Index)

	for _, addr := range addrs {
		if err := routerRemoveRoutes(link, addr); err != nil {
			return fmt.Errorf("failed to remove route: %w", err)
		}

		if err := routerRemoveRules(link, addr); err != nil {
			return fmt.Errorf("failed to remove rule: %w", err)
		}

		if len(tcBindIface) > 0 && !unmanaged {
			if err := tcRemoveFilters(tcBindIface, linkID.ClassID(), addr); err != nil {
				return err
			}
		}
	}

	return nil
}

func RouterAnnounceGateways(linkname string, addrs []string, gateway4 string) error {
	link, err := netlink.LinkByName(linkname)
	if err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

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

//
// QoS functions
//

func RouterSetInboundLimits(linkname string, rateMbit uint32) error {
	link, err := netlink.LinkByName(linkname)
	if err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

	return routerSetInboundLimits(link, rateMbit)
}

func routerSetInboundLimits(link netlink.Link, rateMbit uint32) error {
	ensureLink(link)

	linkname := link.Attrs().Name

	// We don't care about possible errors
	tcRemoveQdisc(linkname)

	if rateMbit == 0 {
		return nil
	}

	// Make a new qdisc on interface if not exists
	if err := tcCreateQdisc(linkname); err != nil {
		return err
	}

	if err := tcCreateClass(linkname, netlink.MakeHandle(1, 1), rateMbit); err != nil {
		return err
	}

	return nil
}

func RouterSetOutboundLimits(linkname string, rateMbit uint32, tcBindIface string) error {
	link, err := netlink.LinkByName(linkname)
	if err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

	linkID := GetLinkID(linkname, link.Attrs().Index)

	if _, err := netlink.LinkByName(tcBindIface); err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

	return routerSetOutboundLimits(linkID, rateMbit, tcBindIface)
}

func routerSetOutboundLimits(linkID LinkID, rateMbit uint32, tcBindIface string) error {
	if rateMbit == 0 { // means that the configuration needs to be completely removed
		// filters must be removed first in this case
		tcRemoveFilters(tcBindIface, linkID.ClassID())
	}

	// We don't care about possible errors
	tcRemoveClass(tcBindIface, linkID.ClassID())

	if rateMbit == 0 {
		return nil
	}

	// Make a new qdisc on interface if not exists
	if err := tcCreateQdisc(tcBindIface); err != nil {
		return err
	}

	if err := tcCreateClass(tcBindIface, linkID.ClassID(), rateMbit); err != nil {
		return err
	}

	return nil
}

//
// ip-route / ip-addr functions
//

func routerAddRoute(link netlink.Link, addr, table string) error {
	ensureLink(link)

	tableNum, err := utils.GetRouteTableIndex(table)
	if err != nil {
		return err
	}

	ipnet, err := utils.ParseIPNet(addr)
	if err != nil {
		return err
	}

	maskOnes, maskBits := ipnet.Mask.Size()

	if ipnet.IP.To4() != nil {
		if maskOnes <= 30 {
			lastIP, _ := ipmath.GetLastIPv4(ipnet)

			gwAddr := netlink.Addr{
				IPNet: &net.IPNet{
					IP:   net.ParseIP(netip.MustParseAddr(lastIP.String()).Prev().String()),
					Mask: net.CIDRMask(maskOnes, maskBits),
				},
			}

			if err := netlink.AddrAdd(link, &gwAddr); err != nil {
				return fmt.Errorf("netlink: unable to add gateway address: %w", err)
			}
		}
	} else {
		if maskOnes <= 64 {
			lastIP, _ := ipmath.GetLastIPv6(ipnet)

			lastAddr := netlink.Addr{
				IPNet: &net.IPNet{
					IP:   lastIP,
					Mask: net.CIDRMask(maskOnes, maskBits),
				},
			}

			if err := netlink.AddrAdd(link, &lastAddr); err != nil {
				return fmt.Errorf("netlink: unable to add gateway address: %w", err)
			}
		} else {
			return fmt.Errorf("too small IPv6 netmask")
		}
	}

	_, dst, err := utils.ParseCIDR(addr)
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
		return fmt.Errorf("netlink: %w", err)
	}

	return nil
}

func routerRemoveRoutes(link netlink.Link, addrs ...string) error {
	ensureLink(link)

	routes, err := netlink.RouteList(link, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

	// We should also delete gateway addresses from the link
	ifacesAddrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

	var candidates []netlink.Route

	if len(addrs) == 0 {
		candidates = routes
	} else {
		dstPrefixes := make(map[string]struct{})

		// Normalize specified addr list
		for _, addr := range addrs {
			_, dst, err := utils.ParseCIDR(addr)
			if err != nil {
				return err
			}

			dstPrefixes[dst.String()] = struct{}{}
		}

		candidates = make([]netlink.Route, 0, len(addrs))

		for _, route := range routes {
			if _, ok := dstPrefixes[route.Dst.String()]; ok {
				candidates = append(candidates, route)
			}
		}
	}

	for _, route := range candidates {
		if route.Dst.IP.IsLinkLocalUnicast() || route.Dst.IP.IsLinkLocalMulticast() {
			continue
		}

		for _, addr := range ifacesAddrs {
			if route.Dst.Contains(addr.IP) {
				netlink.AddrDel(link, &addr)
			}
		}

		netlink.RouteDel(&route)
	}

	return nil
}

//
// ip-rule functions
//

func routerAddRule(link netlink.Link, addr, table string) error {
	ensureLink(link)

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
		return fmt.Errorf("netlink: %w", err)
	}

	return nil
}

func routerRemoveRules(link netlink.Link, prefixes ...string) error {
	ensureLink(link)

	linkname := link.Attrs().Name

	/*
		TODO: see https://github.com/vishvananda/netlink/issues/838 for details.
	*/
	rules, err := netlink.RuleList(netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("netlink: %w", err)
	}

	var candidates []netlink.Rule

	if len(prefixes) == 0 {
		candidates = rules
	} else {
		normalized := make(map[string]struct{})

		// Normalize specified prefixes list
		for _, addr := range prefixes {
			ipnet, err := utils.ParseIPNet(addr)
			if err != nil {
				return err
			}

			normalized[ipnet.String()] = struct{}{}
		}

		candidates = make([]netlink.Rule, 0, len(prefixes))

		for _, rule := range rules {
			if _, ok := normalized[rule.Src.String()]; ok {
				candidates = append(candidates, rule)
			}
		}
	}

	for _, rule := range candidates {
		if rule.IifName == linkname {
			netlink.RuleDel(&rule)
		}
	}

	if len(prefixes) == 0 {
		// Remove all blackhole rules from link
		for _, f := range []string{"inet", "inet6"} {
			args := []string{"-family", f, "rule", "del", "from", "all", "iif", linkname, "blackhole"}

			for {
				if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
					if exitCode, ok := utils.CommandExitCode(err); ok && exitCode == 2 {
						break
					}
					return fmt.Errorf(
						"failed to remove blackhole rule for %s (%w): %s",
						linkname,
						err,
						strings.TrimSpace(string(out)),
					)
				}
			}
		}
	}

	return nil
}

func routerAddBlackholeRules(link netlink.Link) error {
	ensureLink(link)

	/*
		TODO: see https://github.com/vishvananda/netlink/issues/838 for details.
	*/

	linkname := link.Attrs().Name

	for _, f := range []string{"inet", "inet6"} {
		args := []string{"-family", f, "rule", "add", "iif", linkname, "blackhole"}

		if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
			return fmt.Errorf(
				"failed to create blackhole rule for %s (family = %s): %s",
				linkname,
				f,
				strings.TrimSpace(string(out)),
			)
		}
	}

	return nil
}
