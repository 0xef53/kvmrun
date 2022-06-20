package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var channelsCommands = &cli.Command{
	Name:     "channels",
	Usage:    "manage communication channels (vsock, virtio_serial)",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdChannelAttachVSock,
		cmdChannelDetachVSock,
	},
}

var cmdChannelAttachVSock = &cli.Command{
	Name:      "attach-vsock",
	Usage:     "attach a new virtio vsock device",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.UintFlag{Name: "cid", DefaultText: "auto", Usage: "unique guest context `ID`"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, attachVSockChannel)
	},
}

func attachVSockChannel(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.AttachChannelRequest{
		Name: vmname,
		Channel: &pb.AttachChannelRequest_Vsock{
			Vsock: &pb.AttachChannelRequest_VirtioVSock{
				ContextID: uint32(c.Uint("cid")),
			},
		},
		Live: c.Bool("live"),
	}

	_, err := pb.NewMachineServiceClient(conn).AttachChannel(ctx, &req)

	return err
}

var cmdChannelDetachVSock = &cli.Command{
	Name:      "detach-vsock",
	Usage:     "detach an existing virtio vsock device",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, detachVSockChannel)
	},
}

func detachVSockChannel(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.DetachChannelRequest{
		Name: vmname,
		Channel: &pb.DetachChannelRequest_Vsock{
			Vsock: &pb.DetachChannelRequest_VirtioVSock{},
		},
		Live: c.Bool("live"),
	}

	_, err := pb.NewMachineServiceClient(conn).DetachChannel(ctx, &req)

	return err
}
