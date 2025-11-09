package client

import (
	"context"
	"fmt"

	pb_network "github.com/0xef53/kvmrun/api/services/network/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"
	"github.com/0xef53/kvmrun/server/network"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func NetworkSchemeCreateConf(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	ifname := c.Args().Tail()[0]

	req := pb_network.CreateConfRequest{
		Name: vmname,
		Options: &pb_types.NetworkSchemeOpts{
			Ifname: ifname,
		},
	}

	if c.IsSet("mtu") {
		req.Options.MTU = c.Uint32("mtu")
	}

	if c.IsSet("ip") {
		req.Options.Addrs = c.StringSlice("ip")
	}

	req.Options.Gateway4 = c.String("gateway4")
	req.Options.Gateway6 = c.String("gateway6")

	var schemeType network.SchemeType

	if c.Value("type") != nil {
		if v, ok := c.Value("type").(network.SchemeType); ok {
			schemeType = v
		}
	}

	switch schemeType {
	case network.Scheme_ROUTED:
		req.Options.Attrs = &pb_types.NetworkSchemeOpts_Router{
			Router: &pb_types.NetworkSchemeOpts_Attrs_Router{
				BindInterface: c.String("parent-interface"),
				InLimit:       c.Uint32("in-limit"),
				OutLimit:      c.Uint32("out-limit"),
			},
		}
	case network.Scheme_BRIDGE:
		req.Options.Attrs = &pb_types.NetworkSchemeOpts_Bridge{
			Bridge: &pb_types.NetworkSchemeOpts_Attrs_Bridge{
				BridgeName: c.String("bridge-name"),
			},
		}
	case network.Scheme_VXLAN:
		req.Options.Attrs = &pb_types.NetworkSchemeOpts_Vxlan{
			Vxlan: &pb_types.NetworkSchemeOpts_Attrs_VxLAN{
				BindInterface: c.String("parent-interface"),
				VNI:           c.Uint32("vni"),
			},
		}
	case network.Scheme_VLAN:
		req.Options.Attrs = &pb_types.NetworkSchemeOpts_Vlan{
			Vlan: &pb_types.NetworkSchemeOpts_Attrs_VLAN{
				ParentInterface: c.String("parent-interface"),
				VlanID:          c.Uint32("vlan-id"),
			},
		}
	case network.Scheme_MANUAL:
	default:
		return fmt.Errorf("unknown network scheme: %s", schemeType)
	}

	_, err := grpcClient.Network().CreateConf(ctx, &req)

	return err
}

func NetworkSchemeUpdateConf(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	ifname := c.Args().Tail()[0]

	req := pb_network.UpdateConfRequest{
		Name:   vmname,
		Ifname: ifname,
	}

	if c.IsSet("mtu") {
		req.MTU = &pb_network.UpdateConfRequest_MTU{
			Value: c.Uint32("mtu"),
		}
	}

	if c.IsSet("gateway4") {
		req.Gateway4 = &pb_network.UpdateConfRequest_Gateway{
			Value: c.String("gateway4"),
		}
	}

	if c.IsSet("gateway6") {
		req.Gateway6 = &pb_network.UpdateConfRequest_Gateway{
			Value: c.String("gateway6"),
		}
	}

	if c.IsSet("in-limit") {
		req.InLimit = &pb_network.UpdateConfRequest_Limit{
			Value: c.Uint32("in-limit"),
		}
	}

	if c.IsSet("out-limit") {
		req.OutLimit = &pb_network.UpdateConfRequest_Limit{
			Value: c.Uint32("out-limit"),
		}
	}

	if c.IsSet("del-ip") || c.IsSet("add-ip") {
		totalCount := len(c.StringSlice("del-ip")) + len(c.StringSlice("add-ip"))

		req.Addrs = make([]*pb_network.UpdateConfRequest_Addr, 0, totalCount)

		for _, ipstr := range c.StringSlice("del-ip") {
			req.Addrs = append(req.Addrs, &pb_network.UpdateConfRequest_Addr{
				Addr:   ipstr,
				Action: pb_network.UpdateConfRequest_Addr_REMOVE,
			})
		}

		for _, ipstr := range c.StringSlice("add-ip") {
			req.Addrs = append(req.Addrs, &pb_network.UpdateConfRequest_Addr{
				Addr:   ipstr,
				Action: pb_network.UpdateConfRequest_Addr_APPEND,
			})
		}
	}

	_, err := grpcClient.Network().UpdateConf(ctx, &req)

	return err
}

func NetworkSchemeRemoveConf(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	ifname := c.Args().Tail()[0]

	req := pb_network.DeleteConfRequest{
		Name:   vmname,
		Ifname: ifname,
	}

	_, err := grpcClient.Network().DeleteConf(ctx, &req)

	return err
}
