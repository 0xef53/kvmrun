package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"

	cli "github.com/urfave/cli/v3"
)

var VNCCommands = &cli.Command{
	Name:     "vnc",
	Usage:    "manage VNC settings",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		cmdVNCActivate,
	},
}

var cmdVNCActivate = &cli.Command{
	Name:      "activate",
	Usage:     "set VNC password",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "password", Aliases: []string{"p"}, Usage: "`secret` passphrase"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineActivateVNC)
	},
}
