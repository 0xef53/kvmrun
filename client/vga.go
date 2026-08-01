package client

import (
	"context"
	"fmt"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"
	"github.com/0xef53/kvmrun/kvmrun"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func MachineVgaParametersSet(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	var err error

	fmt.Printf("client/MachineVgaParametersSet: c = %v\n", c)
	fmt.Printf("client/MachineVgaParametersSet: c.Value(type) = %v\n", c.Value("type"))
	if c.Value("type") != nil {
		req := pb_machines.VGADeviceSetTypeRequest{
			Name: vmname,
		}

		fmt.Printf("client/MachineVgaParametersSet: type of c = %T\n", c.Value("type"))
		if v, ok := c.Value("type").(kvmrun.QemuVgaType); ok {
			req.Type = pb_types.VGADeviceType(v)
		}
		fmt.Printf("client/MachineVgaParametersSet: req.Type = %v\n", req.Type)

		_, err = grpcClient.Machines().VGADeviceSetType(ctx, &req)
	}

	return err
}
