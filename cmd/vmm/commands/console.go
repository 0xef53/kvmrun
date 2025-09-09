package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"

	cli "github.com/urfave/cli/v3"
)

var CommandConsole = &cli.Command{
	Name:      "console",
	Usage:     "connect to a virtual machine console",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Configuration",
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineConsoleConnect)
	},
}
