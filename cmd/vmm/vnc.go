package main

import (
	"context"
	"encoding/json"
	"fmt"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var vncCommands = &cli.Command{
	Name:     "vnc",
	Usage:    "manage VNC settings",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdVNCActivate,
	},
}

var cmdVNCActivate = &cli.Command{
	Name:      "activate",
	Usage:     "set VNC password",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "password", Aliases: []string{"p"}, Usage: "`secret` passphrase"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, activateVNC)
	},
}

func activateVNC(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.ActivateVNCRequest{
		Name:     vmname,
		Password: c.String("p"),
	}

	resp, err := pb.NewMachineServiceClient(conn).ActivateVNC(ctx, &req)
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
