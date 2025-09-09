package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"

	cli "github.com/urfave/cli/v3"
)

var CommandStart = &cli.Command{
	Name:      "start",
	Usage:     "start a virtual machine process",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Control",
	Flags: []cli.Flag{
		&cli.UintFlag{Name: "wait", Aliases: []string{"w"}, Usage: "wait up to a given `seconds` for the command to take effect"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineStart)
	},
}

var CommandStop = &cli.Command{
	Name:      "stop",
	Usage:     "stop a running virtual machine",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Control",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "wait", Aliases: []string{"w"}, Usage: "block until the operation completes"},
		&cli.BoolFlag{Name: "force", Usage: "stop a virtual machine immediately"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineStop)
	},
}

var CommandRestart = &cli.Command{
	Name:      "restart",
	Usage:     "restart a running virtual machine",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Control",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "wait", Aliases: []string{"w"}, Usage: "block until the operation completes"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineRestart)
	},
}

var CommandReset = &cli.Command{
	Name:      "reset",
	Usage:     "reset a running virtual machine",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Control",
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineReset)
	},
}
