package client

import (
	"context"

	"github.com/0xef53/kvmrun/internal/pci"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"

	cli "github.com/urfave/cli/v3"
)

func MachineHostDeviceAttach(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	addr, err := pci.AddressFromHex(c.Args().Tail()[0])
	if err != nil {
		return err
	}

	req := pb_machines.HostDeviceAttachRequest{
		Name:       vmname,
		PCIAddr:    addr.String(),
		StrictMode: !c.Bool("no-check-device"),
	}

	if c.Value("multifunction") != nil {
		if v, ok := c.Value("multifunction").(bool); ok {
			req.Multifunction = v
		}
	}

	if c.Value("primary-gpu") != nil {
		if v, ok := c.Value("primary-gpu").(bool); ok {
			req.PrimaryGPU = v
		}
	}

	_, err = grpcClient.Machines().HostDeviceAttach(ctx, &req)

	return err
}

func MachineHostDeviceDetach(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	addr, err := pci.AddressFromHex(c.Args().Tail()[0])
	if err != nil {
		return err
	}

	req := pb_machines.HostDeviceDetachRequest{
		Name:    vmname,
		PCIAddr: addr.String(),
	}

	_, err = grpcClient.Machines().HostDeviceDetach(ctx, &req)

	return err
}

func MachineHostDeviceSetOptions(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	addr, err := pci.AddressFromHex(c.Args().Tail()[0])
	if err != nil {
		return err
	}

	if c.IsSet("multifunction") {
		req := pb_machines.HostDeviceSetMultifunctionOptionRequest{
			Name:    vmname,
			PCIAddr: addr.String(),
		}

		if c.Value("multifunction") != nil {
			if v, ok := c.Value("multifunction").(bool); ok {
				req.Enabled = v
			}
		}

		if _, err := grpcClient.Machines().HostDeviceSetMultifunctionOption(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("primary-gpu") {
		req := pb_machines.HostDeviceSetPrimaryGPUOptionRequest{
			Name:    vmname,
			PCIAddr: addr.String(),
		}

		if c.Value("primary-gpu") != nil {
			if v, ok := c.Value("primary-gpu").(bool); ok {
				req.Enabled = v
			}
		}

		if _, err := grpcClient.Machines().HostDeviceSetPrimaryGPUOption(ctx, &req); err != nil {
			return err
		}
	}

	return nil
}
