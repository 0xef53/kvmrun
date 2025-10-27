package client

import (
	"context"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func MachineInputDeviceAttach(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.InputDeviceAttachRequest{
		Name: vmname,
	}

	if c.Value("type") != nil {
		if v, ok := c.Value("type").(pb_types.InputDeviceType); ok {
			req.Type = v
		}
	}

	_, err := grpcClient.Machines().InputDeviceAttach(ctx, &req)

	return err
}

func MachineInputDeviceDetach(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.InputDeviceDetachRequest{
		Name: vmname,
	}

	if c.Value("type") != nil {
		if v, ok := c.Value("type").(pb_types.InputDeviceType); ok {
			req.Type = v
		}
	}

	_, err := grpcClient.Machines().InputDeviceDetach(ctx, &req)

	return err
}
