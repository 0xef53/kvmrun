package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"
	"github.com/0xef53/kvmrun/client/flag_types"

	grpc_client "github.com/0xef53/kvmrun/client/grpcclient"

	cli "github.com/urfave/cli/v3"
)

var InputDeviceCommands = &cli.Command{
	Name:     "inputs",
	Usage:    "manage input devices (usb-tablet)",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		CommandInputDeviceAttach,
		CommandInputDeviceDetach,
	},
}

var CommandInputDeviceAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a new input device",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "type", Aliases: []string{"t"}, Value: flag_types.DefaultInputDeviceType(), Usage: "input device `type`"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.MachineInputDeviceAttach)
	},
}

var CommandInputDeviceDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach an existing input device",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "type", Aliases: []string{"t"}, Value: flag_types.DefaultInputDeviceType(), Usage: "input device `type`"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.MachineInputDeviceDetach)
	},
}
