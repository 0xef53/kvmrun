package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var extkernelCommands = &cli.Command{
	Name:     "kernel",
	Usage:    "manage external kernel parameters",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdExtkernelSet,
	},
}

var cmdExtkernelSet = &cli.Command{
	Name:      "set",
	Usage:     "set various parameters of external kernel",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "image", Value: "-1", DefaultText: "not set", Usage: "kernel image `file` name"},
		&cli.StringFlag{Name: "initrd", Value: "-1", DefaultText: "not set", Usage: "ramdisk image `file` name"},
		&cli.StringFlag{Name: "cmdline", Value: "-1", DefaultText: "not set", Usage: "additional kernel `parameters` (separated by semicolon)"},
		&cli.StringFlag{Name: "modiso", Value: "-1", DefaultText: "not set", Usage: "name of iso `image` with modules"},
		&cli.BoolFlag{Name: "remove-conf", Usage: "remove an existing configuration"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, setExtkernelParameters)
	},
}

func setExtkernelParameters(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.SetExternalKernelRequest{
		Name:       vmname,
		Image:      c.String("image"),
		Initrd:     c.String("initrd"),
		Cmdline:    c.String("cmdline"),
		Modiso:     c.String("modiso"),
		RemoveConf: c.Bool("remove-conf"),
	}

	_, err := pb.NewMachineServiceClient(conn).SetExternalKernel(ctx, &req)

	return err
}
