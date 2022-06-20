package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var cmdStart = &cli.Command{
	Name:      "start",
	Usage:     "start a virtual machine process",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Control",
	Flags: []cli.Flag{
		&cli.UintFlag{Name: "wait", Aliases: []string{"w"}, Usage: "wait up to a given `seconds` for the command to take effect"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, startMachine)
	},
}

func startMachine(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.StartMachineRequest{
		Name:         vmname,
		WaitInterval: int32(c.Uint("wait")),
	}

	_, err := pb.NewMachineServiceClient(conn).Start(ctx, &req)

	return err
}

var cmdStop = &cli.Command{
	Name:      "stop",
	Usage:     "stop a running virtual machine",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Control",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "wait", Aliases: []string{"w"}, Usage: "block until the operation completes"},
		&cli.BoolFlag{Name: "force", Usage: "stop a virtual machine immediately"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, stopMachine)
	},
}

func stopMachine(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.StopMachineRequest{
		Name:  vmname,
		Wait:  c.Bool("wait"),
		Force: c.Bool("force"),
	}

	_, err := pb.NewMachineServiceClient(conn).Stop(ctx, &req)

	return err
}

var cmdRestart = &cli.Command{
	Name:      "restart",
	Usage:     "restart a running virtual machine",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Control",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "wait", Aliases: []string{"w"}, Usage: "block until the operation completes"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, restartMachine)
	},
}

func restartMachine(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.RestartMachineRequest{
		Name: vmname,
		Wait: c.Bool("wait"),
	}

	_, err := pb.NewMachineServiceClient(conn).Restart(ctx, &req)

	return err
}

var cmdReset = &cli.Command{
	Name:      "reset",
	Usage:     "reset a running virtual machine",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Control",
	Action: func(c *cli.Context) error {
		return executeGRPC(c, resetMachine)
	},
}

func resetMachine(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.RestartMachineRequest{
		Name: vmname,
	}

	_, err := pb.NewMachineServiceClient(conn).Reset(ctx, &req)

	return err
}
