package client

import (
	"context"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"

	cli "github.com/urfave/cli/v3"
)

func MachineCdromAttach(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	req := pb_machines.CdromAttachRequest{
		Name:        vmname,
		DeviceName:  c.Args().Tail()[0],
		DeviceMedia: c.String("media"),
		Position:    int32(c.Int("position")),
		Bootindex:   int32(c.Int("bootindex")),
		Readonly:    c.Bool("read-only"),
		Live:        c.Bool("live"),
	}

	if c.Value("driver") != nil {
		if v, ok := c.Value("driver").(pb_types.CdromDriver); ok {
			req.Driver = v
		}
	}

	_, err := grpcClient.Machines().CdromAttach(ctx, &req)

	return err
}

func MachineCdromDetach(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	req := pb_machines.CdromDetachRequest{
		Name:       vmname,
		DeviceName: c.Args().Tail()[0],
		Live:       c.Bool("live"),
	}

	_, err := grpcClient.Machines().CdromDetach(ctx, &req)

	return err
}

func MachineCdromChangeMedia(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	if c.IsSet("media") {
		req := pb_machines.CdromChangeMediaRequest{
			Name:        vmname,
			DeviceName:  c.Args().Tail()[0],
			DeviceMedia: c.String("media"),
			Live:        c.Bool("live"),
		}

		if _, err := grpcClient.Machines().CdromChangeMedia(ctx, &req); err != nil {
			return err
		}
	}

	return nil
}

func MachineCdromRemoveMedia(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	if c.IsSet("remove-media") && c.Bool("remove-media") {
		req := pb_machines.CdromRemoveMediaRequest{
			Name:       vmname,
			DeviceName: c.Args().Tail()[0],
		}

		_, err := grpcClient.Machines().CdromRemoveMedia(ctx, &req)

		return err
	}

	return nil
}
