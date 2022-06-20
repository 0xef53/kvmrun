package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	flag_types "github.com/0xef53/kvmrun/cmd/vmm/types"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var cdromCommands = &cli.Command{
	Name:     "cdrom",
	Usage:    "manage cdrom devices (attach, detach, change media)",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdCdromAttach,
		cmdCdromDetach,
		cmdCdromSet,
	},
}

var cmdCdromAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a new cdrom device with media",
	ArgsUsage: "VMNAME CDROM MEDIA",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "driver", Value: flag_types.DefaultCdromDriver(), Usage: "device driver `name`"},
		&cli.IntFlag{Name: "index", Value: -1, DefaultText: "not set", Usage: "device index `number` (0 -- is bootable device)"},
		&cli.BoolFlag{Name: "read-only", Aliases: []string{"r"}, Usage: "make device read-only"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
		&cli.StringFlag{Name: "proxy", DefaultText: "not set", Usage: "`path` to a proxy binary file that will be launched for this cdrom media"},
		&cli.GenericFlag{Name: "proxy-env", Aliases: []string{"e"}, Value: flag_types.NewStringMap("="), DefaultText: "not set", Usage: "proxy environment variables (key=value)"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, attachCdrom)
	},
}

func attachCdrom(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.AttachCdromRequest{
		Name:         vmname,
		DeviceName:   c.Args().Tail()[0],
		DeviceMedia:  c.Args().Tail()[1],
		Driver:       c.Generic("driver").(*flag_types.CdromDriver).Value(),
		Index:        int32(c.Int("index")),
		ReadOnly:     c.Bool("read-only"),
		ProxyCommand: c.String("proxy"),
		ProxyEnvs:    c.Generic("proxy-env").(*flag_types.StringMap).Value(),
		Live:         c.Bool("live"),
	}

	_, err := pb.NewMachineServiceClient(conn).AttachCdrom(ctx, &req)

	return err
}

var cmdCdromDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach an existing cdrom device",
	ArgsUsage: "VMNAME CDROM",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, detachCdrom)
	},
}

func detachCdrom(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.DetachCdromRequest{
		Name:       vmname,
		DeviceName: c.Args().Tail()[0],
		Live:       c.Bool("live"),
	}

	_, err := pb.NewMachineServiceClient(conn).DetachCdrom(ctx, &req)

	return err
}

var cmdCdromSet = &cli.Command{
	Name:      "set",
	Usage:     "set various cdrom parameters",
	ArgsUsage: "VMNAME CDROM",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "media", Value: "-1", DefaultText: "not set", Usage: "`path` to a new image to be inserted"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
		&cli.StringFlag{Name: "proxy", DefaultText: "not set", Usage: "`path` to a proxy binary file that will be launched for this cdrom media"},
		&cli.GenericFlag{Name: "proxy-env", Aliases: []string{"e"}, Value: flag_types.NewStringMap("="), DefaultText: "not set", Usage: "proxy environment variables (key=value)"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, setCdromParameters)
	},
}

func setCdromParameters(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	client := pb.NewMachineServiceClient(conn)

	if c.IsSet("media") {
		req := pb.ChangeCdromMediaRequest{
			Name:         vmname,
			DeviceName:   c.Args().Tail()[0],
			DeviceMedia:  c.String("media"),
			ProxyCommand: c.String("proxy"),
			ProxyEnvs:    c.Generic("proxy-env").(*flag_types.StringMap).Value(),
			Live:         c.Bool("live"),
		}

		if _, err := client.ChangeCdromMedia(ctx, &req); err != nil {
			return err
		}
	}

	return nil
}
