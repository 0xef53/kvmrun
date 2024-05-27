package main

import (
	"context"
	"encoding/json"
	"fmt"

	h_pb "github.com/0xef53/kvmrun/api/services/hardware/v1"
	t_pb "github.com/0xef53/kvmrun/api/services/tasks/v1"

	empty "github.com/golang/protobuf/ptypes/empty"
	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var systemCommands = &cli.Command{
	Name:     "system",
	Usage:    "manage kvmrund daemon",
	Hidden:   true,
	HideHelp: true,
	Category: "System",
	Subcommands: []*cli.Command{
		cmdPrintTasks,
		cmdPrintPCI,
	},
}

var cmdPrintTasks = &cli.Command{
	Name:     "tasks",
	Usage:    "print a list of background tasks",
	HideHelp: true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, printBackgroundTasks)
	},
}

func printBackgroundTasks(ctx context.Context, _ string, c *cli.Context, conn *grpc.ClientConn) error {
	resp, err := t_pb.NewTaskServiceClient(conn).List(ctx, new(empty.Empty))
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", b)

	return nil
}

var cmdPrintPCI = &cli.Command{
	Name:     "pci-devices",
	Usage:    "print a list of host PCI devices",
	HideHelp: true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, printPCIDevices)
	},
}

func printPCIDevices(ctx context.Context, _ string, c *cli.Context, conn *grpc.ClientConn) error {
	resp, err := h_pb.NewHardwareServiceClient(conn).ListPCI(ctx, new(h_pb.ListPCIRequest))
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", b)

	return nil
}
