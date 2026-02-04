package client

import (
	"context"

	pb_mahines "github.com/0xef53/kvmrun/api/services/machines/v2"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func MachineMemorySetParameters(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_mahines.MemorySetLimitsRequest{
		Name:   vmname,
		Actual: uint32(c.Int("actual")),
		Total:  uint32(c.Int("total")),
		Live:   c.Bool("live"),
	}

	_, err := grpcClient.Machines().MemorySetLimits(ctx, &req)

	return err

}

func MachineCPUSetParameters(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	if c.IsSet("actual") || c.IsSet("total") {
		req := pb_mahines.CPUSetLimitsRequest{
			Name:   vmname,
			Actual: uint32(c.Int("actual")),
			Total:  uint32(c.Int("total")),
			Live:   c.Bool("live"),
		}

		if _, err := grpcClient.Machines().CPUSetLimits(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("sockets") {
		req := pb_mahines.CPUSetSocketsRequest{
			Name:    vmname,
			Sockets: uint32(c.Int("sockets")),
		}

		if _, err := grpcClient.Machines().CPUSetSockets(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("quota") {
		req := pb_mahines.CPUSetQuotaRequest{
			Name:  vmname,
			Quota: uint32(c.Int("quota")),
			Live:  c.Bool("live"),
		}

		if _, err := grpcClient.Machines().CPUSetQuota(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("model") {
		req := pb_mahines.CPUSetModelRequest{
			Name:  vmname,
			Model: c.String("model"),
		}

		if _, err := grpcClient.Machines().CPUSetModel(ctx, &req); err != nil {
			return err
		}
	}

	return nil
}
