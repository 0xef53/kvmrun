package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"
	"github.com/0xef53/kvmrun/client/flag_types"

	cli "github.com/urfave/cli/v3"
)

var DiskCommands = &cli.Command{
	Name:     "storage",
	Usage:    "manage storage devices (attach, detach, modify)",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		CommandDiskAttach,
		CommandDiskDetach,
		CommandDiskSetParameters,
		CommandDiskResizeQemuBlockdev,
	},
}

var CommandDiskAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a new disk device",
	ArgsUsage: "VMNAME DISK",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "driver", Value: flag_types.DefaultDiskDriver(), Usage: "block device driver `name`"},
		&cli.IntFlag{Name: "iops-rd", DefaultText: "not set", Usage: "read I/O operations `limit` per second (0 - unlimited)"},
		&cli.IntFlag{Name: "iops-wr", DefaultText: "not set", Usage: "write I/O operations `limit` per second (0 - unlimited)"},
		&cli.IntFlag{Name: "position", Value: -1, DefaultText: "not set", Usage: "position `number` in device list"},
		&cli.IntFlag{Name: "bootindex", Value: 0, DefaultText: "not set", Usage: "boot `priority` for a device (lower value = higher priority)"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineDiskAttach)
	},
}

var CommandDiskDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach an existing disk device",
	ArgsUsage: "VMNAME DISK",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineDiskDetach)
	},
}

var CommandDiskSetParameters = &cli.Command{
	Name:      "set",
	Usage:     "set various disk parameters",
	ArgsUsage: "VMNAME DISK",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.IntFlag{Name: "iops-rd", Value: -1, DefaultText: "not set", Usage: "read I/O operations `limit` per second (0 - unlimited)"},
		&cli.IntFlag{Name: "iops-wr", Value: -1, DefaultText: "not set", Usage: "write I/O operations `limit` per second (0 - unlimited)"},
		&cli.BoolFlag{Name: "remove-bitmap", Usage: "stop write tracking and remove the dirty bitmap (if exists)"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineDiskRemoveQemuBitmap, client.MachineDiskSetReadLimit, client.MachineDiskSetWriteLimit)
	},
}

var CommandDiskResizeQemuBlockdev = &cli.Command{
	Name:      "resize",
	Usage:     "resize a disk device and send an event to the guest",
	ArgsUsage: "VMNAME DISK",
	HideHelp:  true,
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineDiskResizeQemuBlockdev)
	},
}
