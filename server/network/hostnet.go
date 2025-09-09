package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/0xef53/kvmrun/internal/hostnet"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/internal/utils"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

type HostNetworkOptions struct {
	Attrs           HostNetworkAttrs `json:"attrs"`
	WithSecondStage bool             `json:"with_second_stage"`
}

func (o *HostNetworkOptions) Validate(strict bool) error {
	if o.Attrs == nil {
		return fmt.Errorf("empty attrs")
	} else {
		if err := o.Attrs.Validate(strict); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) ConfigureHostNetwork(ctx context.Context, linkName string, opts *HostNetworkOptions) error {
	linkName = strings.TrimSpace(linkName)

	if err := kvmrun.ValidateLinkName(linkName); err != nil {
		return err
	}

	if opts == nil {
		return fmt.Errorf("empty hostnet opts")
	} else {
		if err := opts.Validate(true); err != nil {
			return err
		}
	}

	taskOpts := []task.TaskOption{
		server.WithUniqueLabel(linkName + "/hostnet"),
		server.WithHostnetGroupLabel(),
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(linkName+"/hostnet"), true, taskOpts, func(l *log.Entry) error {
		switch v := opts.Attrs.(type) {
		case *HostNetworAttrs_VLAN:
			attrs := hostnet.VlanDeviceAttrs{
				VlanID: v.VlanID,
				MTU:    uint32(v.MTU),
				Parent: v.ParentIfname,
			}

			return hostnet.ConfigureVlanPort(linkName, &attrs, opts.WithSecondStage)
		case *HostNetworAttrs_VxLAN:
			var ip4 net.IP

			if ips, err := hostnet.ParseBindings(v.BindIfname); err == nil {
				for _, ip := range ips {
					if ip.To4() != nil {
						ip4 = ip

						break
					}
				}
			} else {
				return err
			}

			if ip4 == nil {
				return fmt.Errorf("no IPv4 addresses found on the interface %s", v.BindIfname)
			}

			attrs := hostnet.VxlanDeviceAttrs{
				VNI:   v.VNI,
				MTU:   uint32(v.MTU),
				Local: ip4,
			}

			return hostnet.ConfigureVxlanPort(linkName, &attrs, opts.WithSecondStage)
		case *HostNetworAttrs_Router:
			attrs := hostnet.RouterDeviceAttrs{
				Addrs:          v.Addrs,
				MTU:            uint32(v.MTU),
				BindInterface:  v.BindIfname,
				DefaultGateway: v.DefaultGateway,
				InLimit:        v.InLimit,
				OutLimit:       v.OutLimit,
				ProcessID:      v.ProcessID,
			}

			err := hostnet.ConfigureRouter(linkName, &attrs, opts.WithSecondStage)
			if err != nil && errors.Is(err, hostnet.ErrCgroupBinding) {
				log.WithField("ifname", linkName).Warnf("Non-fatal error: %s", err)

				return nil
			}

			return err
		case *HostNetworAttrs_Bridge:
			attrs := hostnet.BridgeDeviceAttrs{
				Ifname: v.BridgeIfname,
				MTU:    uint32(v.MTU),
			}

			return hostnet.ConfigureBridgePort(linkName, &attrs, opts.WithSecondStage)
		}

		return fmt.Errorf("unknown network scheme")
	})

	if err != nil {
		return fmt.Errorf("cannot configure hostnet backend: %w", err)
	}

	return nil
}

func (s *Server) DeconfigureHostNetwork(ctx context.Context, linkName string, opts *HostNetworkOptions) error {
	linkName = strings.TrimSpace(linkName)

	if err := kvmrun.ValidateLinkName(linkName); err != nil {
		return err
	}

	if opts == nil {
		return fmt.Errorf("empty hostnet opts")
	} else {
		if err := opts.Validate(true); err != nil {
			return err
		}
	}

	taskOpts := []task.TaskOption{
		server.WithUniqueLabel(linkName + "/hostnet"),
		server.WithHostnetGroupLabel(),
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(linkName+"/hostnet"), true, taskOpts, func(l *log.Entry) error {
		switch v := opts.Attrs.(type) {
		case *HostNetworAttrs_VLAN:
			return hostnet.DeconfigureVlanPort(linkName, v.VlanID)
		case *HostNetworAttrs_VxLAN:
			return hostnet.DeconfigureVxlanPort(linkName, v.VNI)
		case *HostNetworAttrs_Router:
			return hostnet.DeconfigureRouter(linkName, v.BindIfname)
		case *HostNetworAttrs_Bridge:
			return hostnet.DeconfigureBridgePort(linkName, v.BridgeIfname)
		}

		return fmt.Errorf("unknown network scheme")
	})

	if err != nil {
		return fmt.Errorf("cannot deconfigure hostnet backend: %w", err)
	}

	return nil
}

type HostNetworkAttrs interface {
	Validate(bool) error
}

type HostNetworAttrs_VLAN struct {
	VlanID       uint32 `json:"vlan_id"`
	MTU          uint16 `json:"mtu"`
	ParentIfname string `json:"parent_ifname"`
}

func (x *HostNetworAttrs_VLAN) Validate(strict bool) error {
	x.ParentIfname = strings.TrimSpace(x.ParentIfname)

	if len(x.ParentIfname) == 0 {
		return fmt.Errorf("empty vlan.parent_ifname value")
	}

	maxVLAN := uint32(1<<(12) - 2)

	if !(x.VlanID > 0 && x.VlanID < maxVLAN) {
		return fmt.Errorf("invalid vlan.vlan_id value: must be greater than 0 and less or equal %d", maxVLAN)
	}

	return nil
}

type HostNetworAttrs_VxLAN struct {
	VNI        uint32 `json:"vni"`
	MTU        uint16 `json:"mtu"`
	BindIfname string `json:"bind_ifname"`
}

func (x *HostNetworAttrs_VxLAN) Validate(strict bool) error {
	x.BindIfname = strings.TrimSpace(x.BindIfname)

	if len(x.BindIfname) == 0 {
		return fmt.Errorf("empty vxlan.bind_ifname value")
	}

	maxVNI := uint32(1<<(24) - 1)

	if !(x.VNI > 0 && x.VNI < maxVNI) {
		return fmt.Errorf("invalid vxlan.vni value: must be greater than 0 and less or equal %d", maxVNI)
	}

	return nil
}

type HostNetworAttrs_Router struct {
	Addrs          []string `json:"addrs"`
	MTU            uint16   `json:"mtu"`
	BindIfname     string   `json:"bind_ifname"`
	DefaultGateway string   `json:"default_gateway"`
	InLimit        uint32   `json:"in_limit"`
	OutLimit       uint32   `json:"out_limit"`
	ProcessID      uint32   `json:"process_id"`
}

func (x *HostNetworAttrs_Router) Validate(strict bool) error {
	for _, addr := range x.Addrs {
		if _, err := utils.ParseIPNet(addr); err != nil {
			return fmt.Errorf("invalid IP address format: %w", err)
		}
	}

	x.BindIfname = strings.TrimSpace(x.BindIfname)

	if len(x.BindIfname) == 0 {
		return fmt.Errorf("empty router.bind_ifname value")
	}

	x.DefaultGateway = strings.TrimSpace(x.DefaultGateway)

	if len(x.DefaultGateway) > 0 {
		if ip := net.ParseIP(x.DefaultGateway); ip == nil {
			return fmt.Errorf("invalid router.default_gateway format: %s", x.DefaultGateway)
		}
	}

	if x.ProcessID == 0 {
		return fmt.Errorf("invalid router.process_id value: must be greater than 0")
	}

	return nil
}

type HostNetworAttrs_Bridge struct {
	BridgeIfname string `json:"ifname"`
	MTU          uint16 `json:"mtu"`
}

func (x *HostNetworAttrs_Bridge) Validate(strict bool) error {
	x.BridgeIfname = strings.TrimSpace(x.BridgeIfname)

	if len(x.BridgeIfname) == 0 {
		return fmt.Errorf("empty bridge.bridge_ifname value")
	}

	return nil
}
