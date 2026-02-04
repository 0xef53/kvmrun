package client

import (
	"context"
	"net"
	"path/filepath"
	"strings"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func MachineNetIfaceAttach(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.NetIfaceAttachRequest{
		Name:         vmname,
		Ifname:       c.Args().Tail()[0],
		Queues:       uint32(c.Uint("queues")),
		IfupScript:   c.String("ifup-script"),
		IfdownScript: c.String("ifdown-script"),
		Live:         c.Bool("live"),
	}

	if c.Value("hwaddr") != nil {
		if v, ok := c.Value("hwaddr").(net.HardwareAddr); ok {
			req.HwAddr = v.String()
		}
	}

	if c.Value("driver") != nil {
		if v, ok := c.Value("driver").(pb_types.NetIfaceDriver); ok {
			req.Driver = v
		}
	}

	_, err := grpcClient.Machines().NetIfaceAttach(ctx, &req)

	return err
}

func MachineNetIfaceDetach(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.NetIfaceDetachRequest{
		Name:   vmname,
		Ifname: c.Args().Tail()[0],
		Live:   c.Bool("live"),
	}

	_, err := grpcClient.Machines().NetIfaceDetach(ctx, &req)

	return err
}

func MachineNetIfaceSetParameters(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	if c.IsSet("queues") {
		req := pb_machines.NetIfaceSetQueuesRequest{
			Name:   vmname,
			Ifname: c.Args().Tail()[0],
			Queues: uint32(c.Uint("queues")),
		}

		if _, err := grpcClient.Machines().NetIfaceSetQueues(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("ifup-script") {
		req := pb_machines.NetIfaceSetScriptRequest{
			Name:   vmname,
			Ifname: c.Args().Tail()[0],
		}

		if script := strings.TrimSpace(c.String("ifup-script")); len(script) > 0 {
			if p, err := filepath.Abs(script); err == nil {
				req.Path = p
			} else {
				return err
			}
		}

		if _, err := grpcClient.Machines().NetIfaceSetUpScript(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("ifdown-script") {
		req := pb_machines.NetIfaceSetScriptRequest{
			Name:   vmname,
			Ifname: c.Args().Tail()[0],
		}

		if script := strings.TrimSpace(c.String("ifdown-script")); len(script) > 0 {
			if p, err := filepath.Abs(script); err == nil {
				req.Path = p
			} else {
				return err
			}
		}

		if _, err := grpcClient.Machines().NetIfaceSetDownScript(ctx, &req); err != nil {
			return err
		}
	}

	if c.IsSet("link-state") {
		req := pb_machines.NetIfaceSetLinkStateRequest{
			Name:   vmname,
			Ifname: c.Args().Tail()[0],
		}

		if c.Value("link-state") != nil {
			if v, ok := c.Value("link-state").(uint16); ok {
				req.State = pb_types.NetIfaceLinkState(v)
			}
		}

		if _, err := grpcClient.Machines().NetIfaceSetLinkState(ctx, &req); err != nil {
			return err
		}
	}

	return nil
}
