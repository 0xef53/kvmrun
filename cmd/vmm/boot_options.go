package main

import (
	"context"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var bootCommands = &cli.Command{
	Name:     "boot",
	Usage:    "manage machine boot parameters (bios/uefi mode)",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdBootSet,
	},
}

var cmdBootSet = &cli.Command{
	Name:      "set",
	Usage:     "set various boot parameters",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "firmware", Value: "", DefaultText: "not set", Usage: "firmware image file `file`"},
		&cli.StringFlag{Name: "flash-device", Value: "", DefaultText: "not set", Usage: "firmware flash device `file`"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, setBootParameters)
	},
}

func setBootParameters(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	if image := c.String("firmware"); len(image) > 0 {
		req := pb.SetFirmwareRequest{
			Name:       vmname,
			Image:      image,
			RemoveConf: strings.ToLower(image) == "default",
		}

		if flash := c.String("flash-device"); len(flash) > 0 {
			req.Flash = flash
		}

		if _, err := pb.NewMachineServiceClient(conn).SetFirmware(ctx, &req); err != nil {
			return err
		}
	}

	return nil
}
