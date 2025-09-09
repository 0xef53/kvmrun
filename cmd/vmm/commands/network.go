package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"
	"github.com/0xef53/kvmrun/client/flag_types"

	cli "github.com/urfave/cli/v3"
)

var NetworkCommands = &cli.Command{
	Name:     "network",
	Usage:    "manage network parameters (attach, detach, modify)",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		CommandNetIfaceAttach,
		CommandNetIfaceDetach,
		CommandNetIfaceSetParameters,
	},
}

var CommandNetIfaceAttach = &cli.Command{
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
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineNetIfaceAttach)
	},
}

var CommandNetIfaceDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach an existing network device",
	ArgsUsage: "VMNAME IFNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineNetIfaceDetach)
	},
}

var CommandNetIfaceSetParameters = &cli.Command{
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
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineNetIfaceSetParameters)
	},
}
