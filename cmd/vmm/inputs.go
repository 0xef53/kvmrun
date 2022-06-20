package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	flag_types "github.com/0xef53/kvmrun/cmd/vmm/types"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var inputsCommands = &cli.Command{
	Name:     "inputs",
	Usage:    "manage input devices (usb-tablet)",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdInputAttach,
		cmdInputDetach,
	},
}

var cmdInputAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a new input device",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "type", Aliases: []string{"t"}, Value: flag_types.DefaultInputDeviceType(), Usage: "input device `type`"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, attachInputDevice)
	},
}

func attachInputDevice(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.AttachInputDeviceRequest{
		Name: vmname,
		Type: c.Generic("type").(*flag_types.InputDeviceType).Value(),
	}

	_, err := pb.NewMachineServiceClient(conn).AttachInputDevice(ctx, &req)

	return err
}

var cmdInputDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach an existing input device",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "type", Aliases: []string{"t"}, Value: flag_types.DefaultInputDeviceType(), Usage: "input device `type`"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, detachInputDevice)
	},
}

func detachInputDevice(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.DetachInputDeviceRequest{
		Name: vmname,
		Type: c.Generic("type").(*flag_types.InputDeviceType).Value(),
	}

	_, err := pb.NewMachineServiceClient(conn).DetachInputDevice(ctx, &req)

	return err
}
