package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"

	cli "github.com/urfave/cli/v3"
)

var BootCommands = &cli.Command{
	Name:     "boot",
	Usage:    "manage machine boot parameters (bios/uefi mode)",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		CommandBootSetParameters,
	},
}

var CommandBootSetParameters = &cli.Command{
	Name:      "set",
	Usage:     "set various boot parameters",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "firmware", Value: "", DefaultText: "not set", Usage: "firmware image file `file`"},
		&cli.StringFlag{Name: "flash-device", Value: "", DefaultText: "not set", Usage: "firmware flash device `file`"},
		&cli.BoolFlag{Name: "remove-conf", Usage: "remove an existing configuration"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.BootParametersSet)
	},
}
