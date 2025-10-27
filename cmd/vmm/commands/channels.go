package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"

	grpc_client "github.com/0xef53/kvmrun/client/grpcclient"

	cli "github.com/urfave/cli/v3"
)

var ChannelCommands = &cli.Command{
	Name:     "channels",
	Usage:    "manage communication channels (vsock, virtio_serial)",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		CommandChannelAttach_VSock,
		CommandChannelDetach_VSock,
	},
}

var CommandChannelAttach_VSock = &cli.Command{
	Name:      "attach-vsock",
	Usage:     "attach a new virtio vsock device",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.UintFlag{Name: "cid", DefaultText: "auto", Usage: "unique guest context `ID`"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.MachineChannelAttach_VSock)
	},
}

var CommandChannelDetach_VSock = &cli.Command{
	Name:      "detach-vsock",
	Usage:     "detach an existing virtio vsock device",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.MachineChannelDetach_VSock)
	},
}
