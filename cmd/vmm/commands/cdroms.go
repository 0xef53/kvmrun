package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"
	"github.com/0xef53/kvmrun/client/flag_types"

	cli "github.com/urfave/cli/v3"
)

var CdromCommands = &cli.Command{
	Name:     "cdrom",
	Usage:    "manage cdrom devices (attach, detach, change media)",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		CommandCdromAttach,
		CommandCdromDetach,
		CommandCdromSetParameters,
	},
}

var CommandCdromAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a new cdrom device with media",
	ArgsUsage: "VMNAME CDROM",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "driver", Value: flag_types.DefaultCdromDriver(), Usage: "device driver `name`"},
		&cli.IntFlag{Name: "position", Value: -1, DefaultText: "not set", Usage: "position `number` in device list"},
		&cli.IntFlag{Name: "bootindex", Value: 0, DefaultText: "not set", Usage: "boot `priority` for a device (lower value = higher priority)"},
		&cli.StringFlag{Name: "media", Value: "", DefaultText: "not set", Usage: "`path` to an image to be inserted"},
		&cli.BoolFlag{Name: "read-only", Aliases: []string{"r"}, Usage: "make device read-only"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineCdromAttach)
	},
}

var CommandCdromDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach an existing cdrom device",
	ArgsUsage: "VMNAME CDROM",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineCdromDetach)
	},
}

var CommandCdromSetParameters = &cli.Command{
	Name:      "set",
	Usage:     "set various cdrom parameters",
	ArgsUsage: "VMNAME CDROM",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "media", Value: "", DefaultText: "not set", Usage: "`path` to a new image to be inserted"},
		&cli.BoolFlag{Name: "remove-media", Usage: "remove inserted media (if exists)"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineCdromRemoveMedia, client.MachineCdromChangeMedia)
	},
}
