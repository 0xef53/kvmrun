package main

import (
	"context"
	"net"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	flag_types "github.com/0xef53/kvmrun/cmd/vmm/types"
	"github.com/0xef53/kvmrun/internal/ipmath"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var networkCommands = &cli.Command{
	Name:     "network",
	Usage:    "manage network parameters (attach, detach, modify)",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdNetworkAttach,
		cmdNetworkDetach,
		cmdNetworkSet,
	},
}

var cmdNetworkAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a new network device",
	ArgsUsage: "VMNAME IFNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "driver", Value: flag_types.DefaultNetIfaceDriver(), Usage: "network device driver `name`"},
		&cli.GenericFlag{Name: "hwaddr", Value: new(flag_types.NetIfaceHwAddr), Usage: "hardware `address` of a network interface"},
		&cli.UintFlag{Name: "queues", DefaultText: "not set", Usage: "`number` of RX/TX queue pairs (0 - not set)"},
		&cli.StringFlag{Name: "ifup-script", Usage: "`script` to configure network on the host side after a machine starts"},
		&cli.StringFlag{Name: "ifdown-script", Usage: "`script` to destroy network on the host side after a machine stops"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, attachNetworkInterface)
	},
}

func attachNetworkInterface(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.AttachNetIfaceRequest{
		Name:         vmname,
		Ifname:       c.Args().Tail()[0],
		Driver:       c.Generic("driver").(*flag_types.NetIfaceDriver).Value(),
		HwAddr:       c.Generic("hwaddr").(*flag_types.NetIfaceHwAddr).String(),
		Queues:       uint32(c.Uint("queues")),
		IfupScript:   c.String("ifup-script"),
		IfdownScript: c.String("ifdown-script"),
		Live:         c.Bool("live"),
	}

	_, err := pb.NewMachineServiceClient(conn).AttachNetIface(ctx, &req)

	return err
}

var cmdNetworkDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach an existing network device",
	ArgsUsage: "VMNAME IFNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, detachNetworkInterface)
	},
}

func detachNetworkInterface(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.DetachNetIfaceRequest{
		Name:   vmname,
		Ifname: c.Args().Tail()[0],
		Live:   c.Bool("live"),
	}

	_, err := pb.NewMachineServiceClient(conn).DetachNetIface(ctx, &req)

	return err
}

var cmdNetworkSet = &cli.Command{
	Name:      "set",
	Usage:     "set various network parameters",
	ArgsUsage: "VMNAME IFNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.UintFlag{Name: "queues", DefaultText: "not set", Usage: "`number` of RX/TX queue pairs (0 - not set)"},
		&cli.StringFlag{Name: "ifup-script", Usage: "`script` to configure network on the host side after a machine starts"},
		&cli.StringFlag{Name: "ifdown-script", Usage: "`script` to destroy network on the host side after a machine stops"},
		&cli.GenericFlag{Name: "link-state", Value: new(flag_types.NetIfaceLinkState), DefaultText: "not set", Usage: "interface link `state` (up or down)"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, setNetworkParameters)
	},
}

func setNetworkParameters(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	client := pb.NewMachineServiceClient(conn)

	if c.IsSet("queues") {
		req := pb.SetNetIfaceQueuesRequest{
			Name:   vmname,
			Ifname: c.Args().Tail()[0],
			Queues: uint32(c.Uint("queues")),
		}

		if _, err := client.SetNetIfaceQueues(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("ifup-script") {
		req := pb.SetNetIfaceScriptRequest{
			Name:   vmname,
			Ifname: c.Args().Tail()[0],
			Path:   c.String("ifup-script"),
		}

		if _, err := client.SetNetIfaceUpScript(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("ifdown-script") {
		req := pb.SetNetIfaceScriptRequest{
			Name:   vmname,
			Ifname: c.Args().Tail()[0],
			Path:   c.String("ifdown-script"),
		}

		if _, err := client.SetNetIfaceDownScript(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("link-state") {
		req := pb.SetNetIfaceLinkRequest{
			Name:   vmname,
			Ifname: c.Args().Tail()[0],
			State:  c.Generic("link-state").(*flag_types.NetIfaceLinkState).Value(),
		}

		if _, err := client.SetNetIfaceLinkState(ctx, &req); err != nil {
			return err
		}
	}

	return nil
}

func parseIPNet(s string) (*net.IPNet, error) {
	if !strings.Contains(s, "/") {
		if net.ParseIP(s).To4() != nil {
			s += "/32"
		} else {
			s += "/128"
		}
	}

	ip, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}

	ipnet.IP = ip

	return ipnet, nil
}

func getGatewayAddr(addr *net.IPNet) net.IP {
	if addr == nil {
		return nil
	}

	ones, bits := addr.Mask.Size()

	if addr.IP.To4() != nil {
		// IPv4
		if ones > 30 || (ones == 0 && bits == 0) {
			return net.IPv4(10, 11, 11, 11)
		}

		gw, _ := ipmath.GetLastIPv4(addr)

		return gw
	}

	// IPv6
	if ones > 64 || (ones == 0 && bits == 0) {
		return nil
	}

	gw, _ := ipmath.GetLastIPv6(addr)

	return gw
}
