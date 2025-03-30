package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	c_pb "github.com/0xef53/kvmrun/api/services/cloudinit/v1"
	m_pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	flag_types "github.com/0xef53/kvmrun/cmd/vmm/types"
	"github.com/0xef53/kvmrun/kvmrun"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var cloudinitCommands = &cli.Command{
	Name:     "cloudinit",
	Usage:    "manage cloud-init drives (attach, detach, build)",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdCloudInitAttach,
		cmdCloudInitDetach,
		cmdCloudInitChangeMedium,
		cmdCloudInitBuild,
	},
}

var cmdCloudInitAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a cloud-init drive",
	ArgsUsage: "VMNAME ISO-IMAGE-FILE",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "driver", Value: flag_types.DefaultCloudInitDriver(), Usage: "cloud-init device driver `name`"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, attachCloudInitDrive)
	},
}

func attachCloudInitDrive(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := m_pb.AttachCloudInitRequest{
		Name:   vmname,
		Driver: c.Generic("driver").(*flag_types.CloudInitDriver).Value(),
	}

	if v, err := filepath.Abs(c.Args().Tail()[0]); err == nil {
		req.Media = v
	} else {
		return err
	}

	_, err := m_pb.NewMachineServiceClient(conn).AttachCloudInitDrive(ctx, &req)

	return err
}

var cmdCloudInitDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach a cloud-init drive",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, detachCloudInitDrive)
	},
}

func detachCloudInitDrive(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := m_pb.DetachCloudInitRequest{
		Name: vmname,
	}

	_, err := m_pb.NewMachineServiceClient(conn).DetachCloudInitDrive(ctx, &req)

	return err
}

var cmdCloudInitChangeMedium = &cli.Command{
	Name:      "change-medium",
	Usage:     "change the medium inserted into a guest system",
	ArgsUsage: "VMNAME ISO-IMAGE-FILE",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "live", Usage: "apply changes to the running machine instance"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, changeCloudInitDrive)
	},
}

func changeCloudInitDrive(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := m_pb.ChangeCloudInitRequest{
		Name: vmname,
		Live: c.Bool("live"),
	}

	if v, err := filepath.Abs(c.Args().Tail()[0]); err == nil {
		req.Media = v
	} else {
		return err
	}

	_, err := m_pb.NewMachineServiceClient(conn).ChangeCloudInitDrive(ctx, &req)

	return err
}

var cmdCloudInitBuild = &cli.Command{
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
	Action: func(c *cli.Context) error {
		return executeGRPC(c, buildCloudInitDrive)
	},
}

func buildCloudInitDrive(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	var outputFile string

	if len(c.Args().Tail()) >= 1 {
		if v, err := filepath.Abs(c.Args().Tail()[0]); err == nil {
			outputFile = v
		} else {
			return err
		}
	} else {
		outputFile = filepath.Join(kvmrun.CONFDIR, vmname, "config_cidata")
	}

	req := c_pb.BuildImageRequest{
		MachineName:      vmname,
		Platform:         c.String("platform"),
		Subplatform:      c.String("subplatform"),
		Cloudname:        c.String("cloudname"),
		Region:           c.String("region"),
		AvailabilityZone: c.String("zone"),
		Hostname:         c.String("hostname"),
		Domain:           c.String("domain"),
		Timezone:         c.String("timezone"),
		OutputFile:       outputFile,
	}

	if fname := strings.TrimSpace(c.String("vendor-config")); len(fname) > 0 {
		b, err := os.ReadFile(fname)
		if err != nil {
			return err
		}

		req.VendorConfig = b
	}

	if fname := strings.TrimSpace(c.String("user-config")); len(fname) > 0 {
		b, err := os.ReadFile(fname)
		if err != nil {
			return err
		}

		req.UserConfig = b
	}

	if _, err := c_pb.NewCloudInitServiceClient(conn).BuildImage(ctx, &req); err != nil {
		return err
	}

	fmt.Println("Image saved to", outputFile)

	return nil
}
