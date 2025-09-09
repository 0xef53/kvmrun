package client

import (
	"context"
	"encoding/json"
	"fmt"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"

	cli "github.com/urfave/cli/v3"
)

func MachineActivateVNC(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	req := pb_machines.VNCActivateRequest{
		Name:     vmname,
		Password: c.String("p"),
	}

	resp, err := grpcClient.Machines().VNCActivate(ctx, &req)
	if err != nil {
		return err
	}

	if c.Bool("json") {
		b, err := json.MarshalIndent(resp.Requisites, "", "    ")
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", b)
	} else {
		fmt.Printf("Password: %s\n", resp.Requisites.Password)
		fmt.Printf("Display/Port: %d/%d\n", resp.Requisites.Display, resp.Requisites.Port)
		fmt.Printf("Websocket port: %d\n", resp.Requisites.WSPort)
	}

	return nil
}
