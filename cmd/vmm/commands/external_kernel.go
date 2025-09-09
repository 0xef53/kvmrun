package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"

	cli "github.com/urfave/cli/v3"
)

var ExternalKernelCommands = &cli.Command{
	Name:     "kernel",
	Usage:    "manage external kernel parameters",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		CommandExternalKernelSetParameters,
	},
}

var CommandExternalKernelSetParameters = &cli.Command{
	Name:      "set",
	Usage:     "set various parameters of external kernel",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "image", DefaultText: "not set", Usage: "kernel image `file` name"},
		&cli.StringFlag{Name: "initrd", DefaultText: "not set", Usage: "ramdisk image `file` name"},
		&cli.StringFlag{Name: "cmdline", DefaultText: "not set", Usage: "additional kernel `parameters` (separated by semicolon)"},
		&cli.StringFlag{Name: "modiso", DefaultText: "not set", Usage: "name of iso `image` with modules"},
		&cli.BoolFlag{Name: "remove-conf", Usage: "remove an existing configuration"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineExternalKernelSetParameters)
	},
}
