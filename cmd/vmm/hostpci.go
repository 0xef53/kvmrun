package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	flag_types "github.com/0xef53/kvmrun/cmd/vmm/types"
	"github.com/0xef53/kvmrun/internal/pci"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var hostpciCommands = &cli.Command{
	Name:     "hostpci",
	Usage:    "manage host PCI devices",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdHostPCIAttach,
		cmdHostPCIDetach,
		cmdHostPCISet,
	},
}

var cmdHostPCIAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a new host PCI device",
	ArgsUsage: "VMNAME PCIADDR",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "multifunction", Value: flag_types.NewStringBool(), Usage: "enable multifunction capability (on/off)"},
		&cli.GenericFlag{Name: "primary-gpu", Value: flag_types.NewStringBool(), Usage: "use as primary GPU instead of standard Cirrus video card (on/off)"},
		&cli.BoolFlag{Name: "no-check-device", Usage: "don't check device accessibility"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, attachHostPCIDevice)
	},
}

func attachHostPCIDevice(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	addr, err := pci.AddressFromHex(c.Args().Tail()[0])
	if err != nil {
		return err
	}

	req := pb.AttachHostPCIDeviceRequest{
		Name:          vmname,
		Addr:          addr.String(),
		Multifunction: c.Generic("multifunction").(*flag_types.StringBool).Value(),
		PrimaryGPU:    c.Generic("primary-gpu").(*flag_types.StringBool).Value(),
		StrictMode:    c.Bool("no-check-device") == false,
	}

	_, err = pb.NewMachineServiceClient(conn).AttachHostPCIDevice(ctx, &req)

	return err
}

var cmdHostPCIDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach an existing host PCI device",
	ArgsUsage: "VMNAME PCIADDR",
	HideHelp:  true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, detachHostPCIDevice)
	},
}

func detachHostPCIDevice(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	addr, err := pci.AddressFromHex(c.Args().Tail()[0])
	if err != nil {
		return err
	}

	req := pb.DetachHostPCIDeviceRequest{
		Name: vmname,
		Addr: addr.String(),
	}

	_, err = pb.NewMachineServiceClient(conn).DetachHostPCIDevice(ctx, &req)

	return err
}

var cmdHostPCISet = &cli.Command{
	Name:      "set",
	Usage:     "set various host PCI device parameters",
	ArgsUsage: "VMNAME PCIADDR",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "multifunction", Value: flag_types.NewStringBool(), Usage: "enable or disable multifunction capability (on/off)"},
		&cli.GenericFlag{Name: "primary-gpu", Value: flag_types.NewStringBool(), Usage: "use as primary GPU instead of standard Cirrus video card (on/off)"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, setHostPCIDeviceOptions)
	},
}

func setHostPCIDeviceOptions(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	addr, err := pci.AddressFromHex(c.Args().Tail()[0])
	if err != nil {
		return err
	}

	client := pb.NewMachineServiceClient(conn)

	if c.IsSet("multifunction") {
		req := pb.SetHostPCIMultifunctionOptionRequest{
			Name:    vmname,
			Addr:    addr.String(),
			Enabled: c.Generic("multifunction").(*flag_types.StringBool).Value(),
		}

		if _, err := client.SetHostPCIMultifunctionOption(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("primary-gpu") {
		req := pb.SetHostPCIPrimaryGPUOptionRequest{
			Name:    vmname,
			Addr:    addr.String(),
			Enabled: c.Generic("primary-gpu").(*flag_types.StringBool).Value(),
		}

		if _, err := client.SetHostPCIPrimaryGPUOption(ctx, &req); err != nil {
			return err
		}
	}

	return nil
}
