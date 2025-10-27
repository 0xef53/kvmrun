package network

import (
	"fmt"
	"net"
	"strings"

	"github.com/0xef53/kvmrun/internal/utils"
	"github.com/0xef53/kvmrun/kvmrun"
)

type NetworkSchemeAttrs interface {
	Validate(bool) error
	Properties() *SchemeProperties
}

type commonAttrs struct {
	Ifname   string   `json:"ifname"`
	MTU      uint32   `json:"mtu,omitempty"`
	Addrs    []string `json:"addrs,omitempty"`
	Gateway4 string   `json:"gateway4"`
	Gateway6 string   `json:"gateway6"`
}

func (x *commonAttrs) Validate(_ bool) error {
	x.Ifname = strings.TrimSpace(x.Ifname)

	if err := kvmrun.ValidateLinkName(x.Ifname); err != nil {
		return err
	}

	addrs := make([]string, 0, len(x.Addrs))

	for _, addr := range x.Addrs {
		if v, err := utils.ParseIPNet(addr); err == nil {
			// normalizing
			addrs = append(addrs, v.String())
		} else {
			return fmt.Errorf("invalid IP address format: %w", err)
		}
	}

	x.Addrs = addrs

	x.Gateway4 = strings.TrimSpace(strings.ToLower(x.Gateway4))
	x.Gateway6 = strings.TrimSpace(strings.ToLower(x.Gateway6))

	if len(x.Gateway4) > 0 && x.Gateway4 != "auto" {
		if ip := net.ParseIP(x.Gateway4); ip != nil {
			if ip.To4() == nil {
				return fmt.Errorf("invalid router.gateway4: not IPv4: %s", ip)
			}

			// normalizing
			x.Gateway4 = ip.String()
		} else {
			return fmt.Errorf("invalid router.gateway4 format: %s", x.Gateway4)
		}
	}

	if len(x.Gateway6) > 0 && x.Gateway6 != "auto" {
		if ip := net.ParseIP(x.Gateway6); ip != nil {
			if ip.To4() != nil {
				return fmt.Errorf("invalid router.gateway6: not IPv6: %s", ip)
			}

			// normalizing
			x.Gateway6 = ip.String()
		} else {
			return fmt.Errorf("invalid router.gateway6 format: %s", x.Gateway6)
		}
	}

	if x.MTU > 0 && x.MTU < 1280 {
		// In IPv6, the minimum link MTU is 1280 octets
		return fmt.Errorf("invalid MTU value: minimum size is 1280")
	}

	return nil
}

func (x *commonAttrs) Properties() *SchemeProperties {
	p := SchemeProperties{
		Ifname: x.Ifname,
	}

	if x.MTU > 0 {
		p.Set("mtu", x.MTU)
	}

	if len(x.Addrs) > 0 {
		p.Set("addrs", x.Addrs)
	}

	if len(x.Gateway4) > 0 {
		p.Set("gateway4", x.Gateway4)
	}

	if len(x.Gateway6) > 0 {
		p.Set("gateway6", x.Gateway6)
	}

	return &p
}

type SchemeType uint16

const (
	Scheme_UNKNOWN SchemeType = iota
	Scheme_MANUAL
	Scheme_ROUTED
	Scheme_BRIDGE
	Scheme_VXLAN
	Scheme_VLAN
)

func (t SchemeType) String() string {
	switch t {
	case Scheme_MANUAL:
		return "manual"
	case Scheme_ROUTED:
		return "routed"
	case Scheme_BRIDGE:
		return "bridge"
	case Scheme_VXLAN:
		return "vxlan"
	case Scheme_VLAN:
		return "vlan"
	}

	return "UNKNOWN"
}

func SchemeTypeValue(s string) SchemeType {
	switch strings.ToLower(s) {
	case "manual":
		return Scheme_MANUAL
	case "routed":
		return Scheme_ROUTED
	case "bridge":
		return Scheme_BRIDGE
	case "vxlan", "bridge-vxlan":
		return Scheme_VXLAN
	case "vlan", "bridge-vlan":
		return Scheme_VLAN
	}

	return Scheme_UNKNOWN
}
