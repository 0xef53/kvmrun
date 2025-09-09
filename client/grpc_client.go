package client

import (
	"context"
	"io"

	pb_cloudinit "github.com/0xef53/kvmrun/api/services/cloudinit/v2"
	pb_hardware "github.com/0xef53/kvmrun/api/services/hardware/v2"
	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_network "github.com/0xef53/kvmrun/api/services/network/v2"
	pb_system "github.com/0xef53/kvmrun/api/services/system/v2"
	pb_tasks "github.com/0xef53/kvmrun/api/services/tasks/v2"

	grpcclient "github.com/0xef53/go-grpc/client"

	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v3"
)

func init() {
	logger := log.New()

	logger.SetOutput(io.Discard)

	grpcclient.SetLogger(log.NewEntry(logger))
}

type kvmrun_Interfaces struct {
	machinesClient  pb_machines.MachineServiceClient
	systemClient    pb_system.SystemServiceClient
	networkClient   pb_network.NetworkServiceClient
	tasksClient     pb_tasks.TaskServiceClient
	cloudinitClient pb_cloudinit.CloudInitServiceClient
	hardwareClient  pb_hardware.HardwareServiceClient
}

func (k *kvmrun_Interfaces) Machines() pb_machines.MachineServiceClient {
	return k.machinesClient
}

func (k *kvmrun_Interfaces) System() pb_system.SystemServiceClient {
	return k.systemClient
}

func (k *kvmrun_Interfaces) Network() pb_network.NetworkServiceClient {
	return k.networkClient
}

func (k *kvmrun_Interfaces) Tasks() pb_tasks.TaskServiceClient {
	return k.tasksClient
}

func (k *kvmrun_Interfaces) CloudInit() pb_cloudinit.CloudInitServiceClient {
	return k.cloudinitClient
}

func (k *kvmrun_Interfaces) Hardware() pb_hardware.HardwareServiceClient {
	return k.hardwareClient
}

func WithGRPC(ctx context.Context, c *cli.Command, fns ...func(context.Context, string, *cli.Command, *kvmrun_Interfaces) error) error {
	if c.Args().Len() < countRequiredArgs(c.ArgsUsage) {
		cli.ShowSubcommandHelpAndExit(c, 1)
	}

	vmname := c.Args().First()

	appConf, err := AppConfFromContext(ctx)
	if err != nil {
		return err
	}

	conn, err := grpcclient.NewSecureConnection("unix:@/run/kvmrund.sock", appConf.TLSConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	K := kvmrun_Interfaces{
		machinesClient:  pb_machines.NewMachineServiceClient(conn),
		systemClient:    pb_system.NewSystemServiceClient(conn),
		networkClient:   pb_network.NewNetworkServiceClient(conn),
		tasksClient:     pb_tasks.NewTaskServiceClient(conn),
		cloudinitClient: pb_cloudinit.NewCloudInitServiceClient(conn),
		hardwareClient:  pb_hardware.NewHardwareServiceClient(conn),
	}

	for _, fn := range fns {
		if err := fn(ctx, vmname, c, &K); err != nil {
			return err
		}
	}

	return nil
}
