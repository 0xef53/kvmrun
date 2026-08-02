package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"
	"github.com/0xef53/kvmrun/client/flag_types"

	grpc_client "github.com/0xef53/kvmrun/client/grpcclient"

	cli "github.com/urfave/cli/v3"
)

var VgaDeviceCommands = &cli.Command{
	Name:     "vga",
	Usage:    "manage VGA card parameters",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		CommandVgaSetParameters,
	},
}

var CommandVgaSetParameters = &cli.Command{
	Name:      "set",
	Usage:     "select type of VGA card to emulate",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "type", Value: flag_types.DefaultVgaDeviceType(), Usage: "`type` of VGA card to emulate (valid values: cirrus, std, virtio-vga)"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.MachineVgaParametersSet)
	},
}
