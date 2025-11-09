package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun"

	pb_cloudinit "github.com/0xef53/kvmrun/api/services/cloudinit/v2"
	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func MachineCloudInitDriveAttach(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.CloudInitDriveAttachRequest{
		Name: vmname,
	}

	if c.Value("driver") != nil {
		if v, ok := c.Value("driver").(pb_types.CloudInitDriver); ok {
			req.Driver = v
		}
	}

	if v, err := filepath.Abs(c.Args().Tail()[0]); err == nil {
		req.Media = v
	} else {
		return err
	}

	_, err := grpcClient.Machines().CloudInitDriveAttach(ctx, &req)

	return err
}

func MachineCloudInitDriveDetach(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.CloudInitDriveDetachRequest{
		Name: vmname,
	}

	_, err := grpcClient.Machines().CloudInitDriveDetach(ctx, &req)

	return err
}

func MachineCloudInitDriveChangeMedia(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.CloudInitDriveChangeMediaRequest{
		Name: vmname,
		Live: c.Bool("live"),
	}

	if v, err := filepath.Abs(c.Args().Tail()[0]); err == nil {
		req.Media = v
	} else {
		return err
	}

	_, err := grpcClient.Machines().CloudInitDriveChangeMedia(ctx, &req)

	return err
}

func MachineCloudInitDriveBuildImage(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
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

	req := pb_cloudinit.BuildImageRequest{
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

	if _, err := grpcClient.CloudInit().BuildImage(ctx, &req); err != nil {
		return err
	}

	fmt.Println("Image saved to", outputFile)

	return nil
}
