package client

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func BootParametersSet(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	if c.IsSet("remove-conf") && c.Bool("remove-conf") {
		req := pb_machines.FirmwareRemoveRequest{
			Name: vmname,
		}

		_, err := grpcClient.Machines().FirmwareRemove(ctx, &req)

		return err
	}

	if image := strings.TrimSpace(c.String("firmware")); len(image) > 0 {
		req := pb_machines.FirmwareSetRequest{
			Name: vmname,
		}

		switch image {
		case "efi", "uefi", "ovmf", "bios", "legacy":
			req.Image = image
		default:
			if p, err := filepath.Abs(image); err == nil {
				req.Image = p
			} else {
				return err
			}
		}

		if flash := strings.TrimSpace(c.String("flash-device")); len(flash) > 0 {
			if p, err := filepath.Abs(flash); err == nil {
				req.Flash = p
			} else {
				return err
			}
		}

		if v, ok := os.LookupEnv("QEMU_ROOTDIR"); ok {
			if v = strings.TrimSpace(v); len(v) != 0 {
				if p, err := filepath.Abs(v); err == nil {
					req.QemuRootdir = p
				} else {
					return err
				}
			}
		}

		_, err := grpcClient.Machines().FirmwareSet(ctx, &req)

		return err
	}

	return nil
}
