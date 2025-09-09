package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"
	"github.com/0xef53/kvmrun/client/flag_types"

	cli "github.com/urfave/cli/v3"
)

var CloudInitCommands = &cli.Command{
	Name:     "cloudinit",
	Usage:    "manage cloud-init drives (attach, detach, build)",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		CommandCloudInitDriveAttach,
		CommandCloudInitDriveDetach,
		CommandCloudInitDriveChangeMedia,
		CommandCloudInitDriveBuildImage,
	},
}

var CommandCloudInitDriveAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a cloud-init drive",
	ArgsUsage: "VMNAME ISO-IMAGE-FILE",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "driver", Value: flag_types.DefaultCloudInitDriver(), Usage: "cloud-init device driver `name`"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineCloudInitDriveAttach)
	},
}

var CommandCloudInitDriveDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach a cloud-init drive",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineCloudInitDriveDetach)
	},
}

var CommandCloudInitDriveChangeMedia = &cli.Command{
	Name:      "change-medium",
	Usage:     "change the medium inserted into a guest system",
	ArgsUsage: "VMNAME ISO-IMAGE-FILE",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "live", Usage: "apply changes to the running machine instance"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineCloudInitDriveChangeMedia)
	},
}

var CommandCloudInitDriveBuildImage = &cli.Command{
	Name:      "build",
	Usage:     "build an ISO image file",
	ArgsUsage: "VMNAME [OUTPUT-FILE]",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "user-config", Usage: "path to the user defined cloud-config data"},
		&cli.StringFlag{Name: "vendor-config", Usage: "path to the vendor defined cloud-config data"},
		&cli.StringFlag{Name: "platform", Usage: "a cloud platform name (nocloud, openstack, gce, ec2, etc...)"},
		&cli.StringFlag{Name: "subplatform", Usage: "additional detail describing the specific source or type of metadata used"},
		&cli.StringFlag{Name: "cloudname", Usage: "a cloud common name (netangels, google, aws, azure, etc...)"},
		&cli.StringFlag{Name: "region", Usage: "the identifier of the region where instances of this platform are located (ru-ekt-1, us-east-2, ...)"},
		&cli.StringFlag{Name: "zone", Usage: "the identifier of the zone in which the instance is deployed (ru-ekt-1a, us-east-2b, ...)"},
		&cli.StringFlag{Name: "hostname", Usage: "set the server hostname"},
		&cli.StringFlag{Name: "domain", Usage: "set the domain to configure FQDN"},
		&cli.StringFlag{Name: "timezone", Usage: "set the system timezone"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return client.WithGRPC(ctx, c, client.MachineCloudInitDriveBuildImage)
	},
}
