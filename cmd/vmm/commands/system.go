package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"

	cli "github.com/urfave/cli/v3"
)

var SystemCommands = &cli.Command{
	Name:     "system",
	Usage:    "manage kvmrund daemon",
	Hidden:   true,
	HideHelp: true,
	Category: "System",
	Commands: []*cli.Command{
		CommandPrintTasks,
		CommandPrintPCI,
	},
}

var CommandPrintTasks = &cli.Command{
	Name:     "tasks",
	Usage:    "print a list of background tasks",
	HideHelp: true,
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.TaskPrintList)
	},
}

var CommandPrintPCI = &cli.Command{
	Name:     "pci-devices",
	Usage:    "print a list of host PCI devices",
	HideHelp: true,
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.PCI_PrintList)
	},
}
