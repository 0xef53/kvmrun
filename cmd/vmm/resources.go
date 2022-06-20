package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var memoryCommands = &cli.Command{
	Name:     "mem-limits",
	Usage:    "manage memory parameters",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdMemorySet,
	},
}

var cmdMemorySet = &cli.Command{
	Name:      "set",
	Usage:     "set various memory parameters",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.IntFlag{Name: "actual", DefaultText: "not set", Usage: "actual memory size in `MiB`"},
		&cli.IntFlag{Name: "total", DefaultText: "not set", Usage: "total/max memory size in `MiB`"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, setMemoryParameters)
	},
}

func setMemoryParameters(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.SetMemLimitsRequest{
		Name:   vmname,
		Actual: int64(c.Int("actual")),
		Total:  int64(c.Int("total")),
		Live:   c.Bool("live"),
	}

	_, err := pb.NewMachineServiceClient(conn).SetMemLimits(ctx, &req)

	return err

}

var cpuCommands = &cli.Command{
	Name:     "cpu-limits",
	Usage:    "manage cpu parameters (quota, model)",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdCPUSet,
	},
}

var cmdCPUSet = &cli.Command{
	Name:      "set",
	Usage:     "set various cpu parameters",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.IntFlag{Name: "actual", DefaultText: "not set", Usage: "actual value of virtual CPU `count`"},
		&cli.IntFlag{Name: "total", DefaultText: "not set", Usage: "total/max value of virtual CPU `count`"},
		&cli.IntFlag{Name: "sockets", DefaultText: "not set", Usage: "`number` of CPU sockets"},
		&cli.IntFlag{Name: "quota", DefaultText: "not set", Usage: "`percent`age of one core"},
		&cli.StringFlag{Name: "model", Usage: "CPU model type (e.g, 'Westmere,+pcid,+ssse3' )"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, setCPUParameters)
	},
}

func setCPUParameters(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	client := pb.NewMachineServiceClient(conn)

	if c.IsSet("actual") || c.IsSet("total") {
		req := pb.SetCPULimitsRequest{
			Name:   vmname,
			Actual: int64(c.Int("actual")),
			Total:  int64(c.Int("total")),
			Live:   c.Bool("live"),
		}

		if _, err := client.SetCPULimits(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("sockets") {
		req := pb.SetCPUSocketsRequest{
			Name:    vmname,
			Sockets: int32(c.Int("sockets")),
		}

		if _, err := client.SetCPUSockets(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("quota") {
		req := pb.SetCPUQuotaRequest{
			Name:  vmname,
			Quota: int32(c.Int("quota")),
			Live:  c.Bool("live"),
		}

		if _, err := client.SetCPUQuota(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("model") {
		req := pb.SetCPUModelRequest{
			Name:  vmname,
			Model: c.String("model"),
		}

		if _, err := client.SetCPUModel(ctx, &req); err != nil {
			return err
		}
	}

	return nil
}
