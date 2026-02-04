package network

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/internal/hostnet"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	"github.com/0xef53/go-task"
	log "github.com/sirupsen/logrus"
)

func (s *Server) DeconfigureHostNetwork(ctx context.Context, vmname, ifname string) error {
	ifname = strings.TrimSpace(ifname)

	if err := kvmrun.ValidateLinkName(ifname); err != nil {
		return err
	}

	taskOpts := []task.TaskOption{
		server.WithUniqueLabel(ifname + "/hostnet"),
		server.WithHostnetGroupLabel(),
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(ifname+"/hostnet"), true, taskOpts, func(l *log.Entry) error {
		/*
			TODO: need to use config_network from the virt.machine chroot
		*/

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

		switch scheme.SchemeType {
		case Scheme_ROUTED:
			attrs, err := scheme.ExtractAttrs_Routed()
			if err != nil {
				return err
			}

			if err := attrs.Validate(true); err != nil {
				return err
			}

			return hostnet.RouterDeconfigure(ifname, attrs.BindInterface)
		case Scheme_BRIDGE:
			attrs, err := scheme.ExtractAttrs_Bridge()
			if err != nil {
				return err
			}

			if err := attrs.Validate(true); err != nil {
				return err
			}

			return hostnet.BridgePortDeconfigure(ifname, attrs.BridgeInterface)
		case Scheme_VXLAN:
			attrs, err := scheme.ExtractAttrs_VxLAN()
			if err != nil {
				return err
			}

			if err := attrs.Validate(true); err != nil {
				return err
			}

			return hostnet.VxlanPortDeconfigure(ifname, attrs.VNI)
		case Scheme_VLAN:
			attrs, err := scheme.ExtractAttrs_VLAN()
			if err != nil {
				return err
			}

			if err := attrs.Validate(true); err != nil {
				return err
			}

			return hostnet.VlanPortDeconfigure(ifname, attrs.VlanID)
		case Scheme_MANUAL:
			return fmt.Errorf("a manual scheme must be deconfigured by your custom scripts")
		}

		return fmt.Errorf("unknown network scheme")
	})

	if err != nil {
		return fmt.Errorf("cannot deconfigure hostnet backend: %w", err)
	}

	return nil
}
