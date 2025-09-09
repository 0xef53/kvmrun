package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"
	"github.com/0xef53/kvmrun/client/flag_types"

	cli "github.com/urfave/cli/v3"
)

var MigrationCommands = &cli.Command{
	Name:     "migration",
	Usage:    "manage migration tasks",
	HideHelp: true,
	Category: "Migration & Backup",
	Commands: []*cli.Command{
		CommandMigrationProcessStart,
		CommandMigrationProcessShowStatus,
		CommandMigrationProcessCancel,
	},
}

var CommandMigrationProcessStart = &cli.Command{
	Name:      "start",
	Usage:     "start migration of the virtual machine to another host",
	ArgsUsage: "VMNAME DSTSERVER",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "watch", Aliases: []string{"w"}, Usage: "watch the migration process"},
		&cli.BoolFlag{Name: "with-local-disks", Usage: "enable live storage migration for all local disks (conflicts with --with-disk)"},
		&cli.StringSliceFlag{Name: "with-disk", Usage: "enable live storage migration for specified disks (conflicts with --with-local-disks)"},
		&cli.StringFlag{Name: "override-name", Value: "", DefaultText: "not set", Usage: "override machine name on the destination server"},
		&cli.GenericFlag{Name: "override-disk", Value: flag_types.NewStringMap(":"), DefaultText: "not set", Usage: "override disk path/name on the destination server"},
		&cli.GenericFlag{Name: "override-net", Value: flag_types.NewStringMap(":"), DefaultText: "not set", Usage: "override net interface name on the destination server"},
		&cli.BoolFlag{Name: "create-disks", Usage: "create logical volumes in the same group on the destination server"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MigrationProcessStart)
	},
}

var CommandMigrationProcessShowStatus = &cli.Command{
	Name:      "status",
	Usage:     "check the progress of a migration",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MigrationProcessShowStatus)
	},
}

var CommandMigrationProcessCancel = &cli.Command{
	Name:      "cancel",
	Usage:     "cancel a running migration process",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MigrationProcessCancel)
	},
}
