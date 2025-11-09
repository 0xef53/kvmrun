package client

import (
	"context"
	"encoding/json"
	"fmt"

	pb_tasks "github.com/0xef53/kvmrun/api/services/tasks/v2"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func TaskPrintList(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	resp, err := grpcClient.Tasks().List(ctx, new(pb_tasks.ListRequest))
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
