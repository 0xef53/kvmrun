package client

import (
	"context"

	pb_mahines "github.com/0xef53/kvmrun/api/services/machines/v2"

	cli "github.com/urfave/cli/v3"
)

func MachineStart(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	req := pb_mahines.StartRequest{
		Name:         vmname,
		WaitInterval: uint32(c.Uint("wait")),
	}

	_, err := grpcClient.Machines().Start(ctx, &req)

	return err
}

func MachineStop(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	req := pb_mahines.StopRequest{
		Name:  vmname,
		Wait:  c.Bool("wait"),
		Force: c.Bool("force"),
	}

	_, err := grpcClient.Machines().Stop(ctx, &req)

	return err
}

func MachineRestart(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	req := pb_mahines.RestartRequest{
		Name: vmname,
		Wait: c.Bool("wait"),
	}

	_, err := grpcClient.Machines().Restart(ctx, &req)

	return err
}

func MachineReset(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	req := pb_mahines.ResetRequest{
		Name: vmname,
	}

	_, err := grpcClient.Machines().Reset(ctx, &req)

	return err
}
