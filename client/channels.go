package client

import (
	"context"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func MachineChannelAttach_VSock(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.ChannelAttachRequest{
		Name: vmname,
		Channel: &pb_machines.ChannelAttachRequest_Vsock{
			Vsock: &pb_machines.ChannelAttachRequest_VirtioVSock{
				ContextID: uint32(c.Uint("cid")),
			},
		},
		Live: c.Bool("live"),
	}

	_, err := grpcClient.Machines().ChannelAttach(ctx, &req)

	return err
}

func MachineChannelDetach_VSock(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.ChannelDetachRequest{
		Name: vmname,
		Channel: &pb_machines.ChannelDetachRequest_Vsock{
			Vsock: &pb_machines.ChannelDetachRequest_VirtioVSock{},
		},
		Live: c.Bool("live"),
	}

	_, err := grpcClient.Machines().ChannelDetach(ctx, &req)

	return err
}
