package main

import (
	"context"
	"encoding/json"
	"fmt"

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
	},
}

var cmdPrintTasks = &cli.Command{
	Name:     "tasks",
	Usage:    "print a list of background tasks",
	HideHelp: true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, listBackgroundTasks)
	},
}

func listBackgroundTasks(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
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
