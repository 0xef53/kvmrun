package client

import (
	"context"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func MachineExternalKernelSetParameters(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	if c.IsSet("remove-conf") && c.Bool("remove-conf") {
		req := pb_machines.ExternalKernelRemoveRequest{
			Name: vmname,
		}

		_, err := grpcClient.Machines().ExternalKernelRemove(ctx, &req)

		return err
	}

	req := pb_machines.ExternalKernelSetRequest{
		Name:    vmname,
		Image:   c.String("image"),
		Initrd:  c.String("initrd"),
		Cmdline: c.String("cmdline"),
		Modiso:  c.String("modiso"),
	}

	_, err := grpcClient.Machines().ExternalKernelSet(ctx, &req)

	return err
}
