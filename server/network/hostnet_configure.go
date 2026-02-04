package network

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xef53/kvmrun/internal/hostnet"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	"github.com/0xef53/go-task"
	log "github.com/sirupsen/logrus"
)

type HostNetworkStage uint16

const (
	ConfigureStage_ALL HostNetworkStage = iota
	ConfifureStage_FIRST
	ConfifureStage_SECOND
)

func (s *Server) ConfigureHostNetwork(ctx context.Context, vmname, ifname string, stage HostNetworkStage) error {
	ifname = strings.TrimSpace(ifname)

	if err := kvmrun.ValidateLinkName(ifname); err != nil {
		return err
	}

	taskOpts := []task.TaskOption{
		server.WithUniqueLabel(ifname + "/hostnet"),
		server.WithHostnetGroupLabel(),
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(ifname+"/hostnet"), true, taskOpts, func(l *log.Entry) error {
		schemes, err := GetNetworkSchemes(vmname)
		if err != nil {
			return err
		}

		var scheme *SchemeProperties

		for _, sc := range schemes {
			if sc.Ifname == ifname {
				scheme = sc

				break
			}
		}

		if scheme == nil {
			return fmt.Errorf("%w: ifname = %s", kvmrun.ErrNotFound, ifname)
		}

		var configureFn func(bool) error

		switch scheme.SchemeType {
		case Scheme_ROUTED:
			attrs, err := scheme.ExtractAttrs_Routed()
			if err != nil {
				return err
			}

			if err := attrs.Validate(true); err != nil {
				return err
			}

			routerAttrs := hostnet.VirtualRouterAttrs{
				BindInterface: attrs.BindInterface,
				MTU:           attrs.MTU,
				Addrs:         attrs.Addrs,
				Gateway4:      attrs.Gateway4,
				Gateway6:      attrs.Gateway6,
				InLimit:       attrs.InLimit,
				OutLimit:      attrs.OutLimit,
			}

			// PID is needed to configure net_cls.classid for use in traffic control rules
			if b, err := os.ReadFile(filepath.Join(kvmrun.CHROOTDIR, vmname, "pid")); err == nil {
				if v, err := strconv.ParseUint(string(b), 10, 32); err == nil {
					routerAttrs.ProcessID = uint32(v)
				} else {
					return err
				}
			} else {
				if errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("%w: %s", kvmrun.ErrNotRunning, vmname)
				}
				return err
			}

			configureFn = func(secondStage bool) error {
				err = hostnet.RouterConfigure(ifname, &routerAttrs, secondStage)

				if err != nil && errors.Is(err, hostnet.ErrCgroupBinding) {
					log.WithField("ifname", ifname).Warnf("Non-fatal error: %s", err)

					return nil
				}

				return err
			}
		case Scheme_BRIDGE:
			attrs, err := scheme.ExtractAttrs_Bridge()
			if err != nil {
				return err
			}

			if err := attrs.Validate(true); err != nil {
				return err
			}

			bridgeAttrs := hostnet.BridgePortAttrs{
				BridgeName: attrs.BridgeInterface,
				MTU:        attrs.MTU,
			}

			configureFn = func(secondStage bool) error {
				return hostnet.BridgePortConfigure(ifname, &bridgeAttrs, secondStage)
			}
		case Scheme_VXLAN:
			attrs, err := scheme.ExtractAttrs_VxLAN()
			if err != nil {
				return err
			}

			if err := attrs.Validate(true); err != nil {
				return err
			}

			var ip4 net.IP

			if ips, err := hostnet.ParseBindings(attrs.BindInterface); err == nil {
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
				return fmt.Errorf("no IPv4 addresses found on interface %s", attrs.BindInterface)
			}

			vxlanAttrs := hostnet.VxlanPortAttrs{
				VNI:   attrs.VNI,
				MTU:   attrs.MTU,
				Local: ip4,
			}

			configureFn = func(secondStage bool) error {
				return hostnet.VxlanPortConfigure(ifname, &vxlanAttrs, secondStage)
			}
		case Scheme_VLAN:
			attrs, err := scheme.ExtractAttrs_VLAN()
			if err != nil {
				return err
			}

			if err := attrs.Validate(true); err != nil {
				return err
			}

			vlanAttrs := hostnet.VlanDeviceAttrs{
				Parent: attrs.ParentInterface,
				VlanID: attrs.VlanID,
				MTU:    attrs.MTU,
			}

			configureFn = func(secondStage bool) error {
				return hostnet.VlanPortConfigure(ifname, &vlanAttrs, secondStage)
			}
		case Scheme_MANUAL:
			return fmt.Errorf("a manual scheme must be configured by your custom scripts")
		default:
			return fmt.Errorf("unknown network scheme")
		}

		switch stage {
		case ConfifureStage_FIRST:
			err = configureFn(false)
		case ConfifureStage_SECOND:
			err = configureFn(true)
		case ConfigureStage_ALL:
			err = configureFn(false)

			if err == nil {
				err = configureFn(true)
			}
		default:
			err = fmt.Errorf("unknown requested stage: %d", stage)
		}

		return err
	})

	if err != nil {
		return fmt.Errorf("cannot configure hostnet backend: %w", err)
	}

	return nil
}
