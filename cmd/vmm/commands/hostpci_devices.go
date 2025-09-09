package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"
	"github.com/0xef53/kvmrun/client/flag_types"

	cli "github.com/urfave/cli/v3"
)

var HostDeviceCommands = &cli.Command{
	Name:     "hostpci",
	Usage:    "manage host PCI devices",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		CommandHostDeviceAttach,
		CommandDeviceDetach,
		CommandDeviceSetOptions,
	},
}

var CommandHostDeviceAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a new host PCI device",
	ArgsUsage: "VMNAME PCIADDR",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "multifunction", Value: flag_types.NewStringBool(), Usage: "enable multifunction capability (on/off)"},
		&cli.GenericFlag{Name: "primary-gpu", Value: flag_types.NewStringBool(), Usage: "use as primary GPU instead of standard Cirrus video card (on/off)"},
		&cli.BoolFlag{Name: "no-check-device", Usage: "don't check device accessibility"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineHostDeviceAttach)
	},
}

var CommandDeviceDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach an existing host PCI device",
	ArgsUsage: "VMNAME PCIADDR",
	HideHelp:  true,
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineHostDeviceDetach)
	},
}

var CommandDeviceSetOptions = &cli.Command{
	Name:      "set",
	Usage:     "set various host PCI device parameters",
	ArgsUsage: "VMNAME PCIADDR",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "multifunction", Value: flag_types.NewStringBool(), Usage: "enable or disable multifunction capability (on/off)"},
		&cli.GenericFlag{Name: "primary-gpu", Value: flag_types.NewStringBool(), Usage: "use as primary GPU instead of standard Cirrus video card (on/off)"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineHostDeviceSetOptions)
	},
}
