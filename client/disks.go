package client

import (
	"context"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func MachineDiskAttach(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.DiskAttachRequest{
		Name:      vmname,
		DiskPath:  c.Args().Tail()[0],
		IopsRd:    uint32(c.Int("iops-rd")),
		IopsWr:    uint32(c.Int("iops-wr")),
		Position:  int32(c.Int("position")),
		Bootindex: int32(c.Int("bootindex")),
		Live:      c.Bool("live"),
	}

	if c.Value("driver") != nil {
		if v, ok := c.Value("driver").(pb_types.DiskDriver); ok {
			req.Driver = v
		}
	}

	_, err := grpcClient.Machines().DiskAttach(ctx, &req)

	return err
}

func MachineDiskDetach(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.DiskDetachRequest{
		Name:     vmname,
		DiskName: c.Args().Tail()[0],
		Live:     c.Bool("live"),
	}

	_, err := grpcClient.Machines().DiskDetach(ctx, &req)

	return err
}

func MachineDiskRemoveQemuBitmap(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	if c.IsSet("remove-bitmap") && c.Bool("remove-bitmap") {
		req := pb_machines.DiskRemoveQemuBitmapRequest{
			Name:     vmname,
			DiskName: c.Args().Tail()[0],
		}

		_, err := grpcClient.Machines().DiskRemoveQemuBitmap(ctx, &req)

		return err
	}

	return nil
}

func MachineDiskSetReadLimit(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	if c.IsSet("iops-rd") {
		req := pb_machines.DiskSetIOLimitRequest{
			Name:     vmname,
			DiskName: c.Args().Tail()[0],
			Iops:     uint32(c.Int("iops-rd")),
			Live:     c.Bool("live"),
		}

		_, err := grpcClient.Machines().DiskSetReadLimit(ctx, &req)

		return err
	}

	return nil
}

func MachineDiskSetWriteLimit(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	if c.IsSet("iops-wr") {
		req := pb_machines.DiskSetIOLimitRequest{
			Name:     vmname,
			DiskName: c.Args().Tail()[0],
			Iops:     uint32(c.Int("iops-wr")),
			Live:     c.Bool("live"),
		}

		_, err := grpcClient.Machines().DiskSetWriteLimit(ctx, &req)

		return err
	}

	return nil
}

func MachineDiskResizeQemuBlockdev(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.DiskResizeQemuBlockdevRequest{
		Name:     vmname,
		DiskName: c.Args().Tail()[0],
	}

	_, err := grpcClient.Machines().DiskResizeQemuBlockdev(ctx, &req)

	return err
}
