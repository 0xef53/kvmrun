package client

import (
	"context"
	"encoding/json"
	"fmt"

	pb_tasks "github.com/0xef53/kvmrun/api/services/tasks/v2"

	cli "github.com/urfave/cli/v3"
)

func TaskPrintList(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
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
