package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"

	grpc_client "github.com/0xef53/kvmrun/client/grpcclient"

	cli "github.com/urfave/cli/v3"
)

var BackupCommands = &cli.Command{
	Name:     "backup",
	Usage:    "manage backup tasks",
	HideHelp: true,
	Category: "Migration & Backup",
	Commands: []*cli.Command{
		CommandBackupProcessStart,
		CommandBackupProcessShowStatus,
		CommandBackupProcessCancel,
	},
}

var CommandBackupProcessStart = &cli.Command{
	Name:      "start",
	Usage:     "start backup of a virtual machine or disk",
	ArgsUsage: "VMNAME [DISKNAME] TARGET",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "watch", Aliases: []string{"w"}, Usage: "watch the process"},
		&cli.BoolFlag{Name: "incremental", Aliases: []string{"inc", "i"}, Usage: "only copy data described by a dirty bitmap (if exists)"},
		&cli.BoolFlag{Name: "clear-bitmap", Usage: "clear/reset a dirty bitmap (if exists) before starting"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.BackupProcessStart)
	},
}

var CommandBackupProcessShowStatus = &cli.Command{
	Name:      "status",
	Usage:     "check the progress of a backup",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.BackupProcessShowStatus)
	},
}

var CommandBackupProcessCancel = &cli.Command{
	Name:      "cancel",
	Usage:     "cancel a running backup process",
	ArgsUsage: "VMNAME [DISKNAME]",
	HideHelp:  true,
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.BackupProcessCancel)
	},
}
