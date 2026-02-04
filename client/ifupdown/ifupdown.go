package ifupdown

import (
	"context"
	"fmt"
	"os"

	"github.com/0xef53/kvmrun/kvmrun"

	pb_network "github.com/0xef53/kvmrun/api/services/network/v2"

	grpc_client "github.com/0xef53/kvmrun/client/grpcclient"
	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	"github.com/vishvananda/netlink"
)

func InterfaceUp(ctx context.Context, ifname string, secondStage bool) error {
	if _, err := netlink.LinkByName(ifname); err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return fmt.Errorf("netlink: link not found: %s", ifname)
		}

		return fmt.Errorf("netlink: %w", err)
	}

	var vmname string

	if cwd, err := os.Getwd(); err == nil {
		if err := kvmrun.ValidateMachineName(cwd); err != nil {
			return err
		}

		vmname = cwd
	} else {
		return fmt.Errorf("cannot determine machine name: %w", err)
	}

	err := grpc_client.KvmrunGRPC(ctx, func(grpcClient *grpc_interfaces.Kvmrun) error {
		req := pb_network.ConfigureRequest{
			Name:        vmname,
			Ifname:      ifname,
			SecondStage: secondStage,
		}

		_, err := grpcClient.Network().Configure(ctx, &req)

		return err
	})

	if err != nil {
		return fmt.Errorf("cannot configure network (ifname = %s): %w", ifname, err)
	}

	return nil
}

func InterfaceDown(ctx context.Context, ifname string) error {
	/*
		TODO: need to use config_network from the virt.machine chroot
	*/

	if _, err := os.Stat("./config_network"); err != nil {
		return err
	}

	var vmname string

	if cwd, err := os.Getwd(); err == nil {
		if err := kvmrun.ValidateMachineName(cwd); err != nil {
			return err
		}

		vmname = cwd
	} else {
		return fmt.Errorf("cannot determine machine name: %w", err)
	}

	err := grpc_client.KvmrunGRPC(ctx, func(grpcClient *grpc_interfaces.Kvmrun) error {
		req := pb_network.DeconfigureRequest{
			Name:   vmname,
			Ifname: ifname,
		}

		_, err := grpcClient.Network().Deconfigure(ctx, &req)

		return err
	})

	if err != nil {
		return fmt.Errorf("cannot deconfigure network (ifname = %s): %w", ifname, err)
	}

	return nil
}
