package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
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
	},
}

var cmdHostPCIAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a new host PCI device",
	ArgsUsage: "VMNAME PCIADDR",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "multifunction", Usage: "enable multifunction capability"},
		&cli.BoolFlag{Name: "primary-gpu", Usage: "use as primary GPU instead of standard Cirrus video card"},
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
		Multifunction: c.Bool("multifunction"),
		PrimaryGPU:    c.Bool("primary-gpu"),
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
