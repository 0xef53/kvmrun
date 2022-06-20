package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	flag_types "github.com/0xef53/kvmrun/cmd/vmm/types"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var storageCommands = &cli.Command{
	Name:     "storage",
	Usage:    "manage storage devices (attach, detach, modify)",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdStorageAttach,
		cmdStorageDetach,
		cmdStorageSet,
		cmdStorageResize,
	},
}

var cmdStorageAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a new disk device",
	ArgsUsage: "VMNAME DISK",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "driver", Value: flag_types.DefaultDiskDriver(), Usage: "block device driver `name`"},
		&cli.IntFlag{Name: "iops-rd", DefaultText: "not set", Usage: "read I/O operations `limit` per second (0 - unlimited)"},
		&cli.IntFlag{Name: "iops-wr", DefaultText: "not set", Usage: "write I/O operations `limit` per second (0 - unlimited)"},
		&cli.IntFlag{Name: "index", Value: -1, DefaultText: "not set", Usage: "disk index `number` (0 -- is bootable device)"},
		&cli.StringFlag{Name: "proxy", DefaultText: "not set", Usage: "`path` to a proxy binary file that will be launched for this disk"},
		&cli.GenericFlag{Name: "proxy-env", Aliases: []string{"e"}, Value: flag_types.NewStringMap("="), DefaultText: "not set", Usage: "proxy environment variables (key=value)"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, attachDisk)
	},
}

func attachDisk(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.AttachDiskRequest{
		Name:         vmname,
		DiskPath:     c.Args().Tail()[0],
		Driver:       c.Generic("driver").(*flag_types.DiskDriver).Value(),
		IopsRd:       int32(c.Int("iops-rd")),
		IopsWr:       int32(c.Int("iops-wr")),
		Index:        int32(c.Int("index")),
		ProxyCommand: c.String("proxy"),
		ProxyEnvs:    c.Generic("proxy-env").(*flag_types.StringMap).Value(),
		Live:         c.Bool("live"),
	}

	_, err := pb.NewMachineServiceClient(conn).AttachDisk(ctx, &req)

	return err
}

var cmdStorageDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach an existing disk device",
	ArgsUsage: "VMNAME DISK",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, detachDisk)
	},
}

func detachDisk(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.DetachDiskRequest{
		Name:     vmname,
		DiskName: c.Args().Tail()[0],
		Live:     c.Bool("live"),
	}

	_, err := pb.NewMachineServiceClient(conn).DetachDisk(ctx, &req)

	return err
}

var cmdStorageSet = &cli.Command{
	Name:      "set",
	Usage:     "set various disk parameters",
	ArgsUsage: "VMNAME DISK",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.IntFlag{Name: "iops-rd", Value: -1, DefaultText: "not set", Usage: "read I/O operations `limit` per second (0 - unlimited)"},
		&cli.IntFlag{Name: "iops-wr", Value: -1, DefaultText: "not set", Usage: "write I/O operations `limit` per second (0 - unlimited)"},
		&cli.BoolFlag{Name: "remove-bitmap", Usage: "stop write tracking and remove the dirty bitmap (if exists)"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, setDiskParameters)
	},
}

func setDiskParameters(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	client := pb.NewMachineServiceClient(conn)

	if c.Bool("remove-bitmap") {
		req := pb.RemoveDiskBitmapRequest{
			Name:     vmname,
			DiskName: c.Args().Tail()[0],
		}

		if _, err := client.RemoveDiskBitmap(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("iops-rd") || c.IsSet("iops-wr") {
		req := pb.SetDiskLimitsRequest{
			Name:     vmname,
			DiskName: c.Args().Tail()[0],
			IopsRd:   int32(c.Int("iops-rd")),
			IopsWr:   int32(c.Int("iops-wr")),
			Live:     c.Bool("live"),
		}

		if _, err := client.SetDiskLimits(ctx, &req); err != nil {
			return err
		}
	}

	return nil
}

var cmdStorageResize = &cli.Command{
	Name:      "resize",
	Usage:     "resize a disk device and send an event to the guest",
	ArgsUsage: "VMNAME DISK",
	HideHelp:  true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, resizeDisk)
	},
}

func resizeDisk(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.ResizeQemuBlockdevRequest{
		Name:     vmname,
		DiskName: c.Args().Tail()[0],
	}

	_, err := pb.NewMachineServiceClient(conn).ResizeQemuBlockdev(ctx, &req)

	return err
}
