package server

import (
	pb_cloudinit "github.com/0xef53/kvmrun/api/services/cloudinit/v2"
	pb_hardware "github.com/0xef53/kvmrun/api/services/hardware/v2"
	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_network "github.com/0xef53/kvmrun/api/services/network/v2"
	pb_system "github.com/0xef53/kvmrun/api/services/system/v2"
	pb_tasks "github.com/0xef53/kvmrun/api/services/tasks/v2"

	grpcclient "github.com/0xef53/go-grpc/client"
)

type Kvmrun_Interfaces struct {
	machinesClient  pb_machines.MachineServiceClient
	systemClient    pb_system.SystemServiceClient
	networkClient   pb_network.NetworkServiceClient
	tasksClient     pb_tasks.TaskServiceClient
	cloudinitClient pb_cloudinit.CloudInitServiceClient
	hardwareClient  pb_hardware.HardwareServiceClient
}

func (k *Kvmrun_Interfaces) Machines() pb_machines.MachineServiceClient {
	return k.machinesClient
}

func (k *Kvmrun_Interfaces) System() pb_system.SystemServiceClient {
	return k.systemClient
}

func (k *Kvmrun_Interfaces) Network() pb_network.NetworkServiceClient {
	return k.networkClient
}

func (k *Kvmrun_Interfaces) Tasks() pb_tasks.TaskServiceClient {
	return k.tasksClient
}

func (k *Kvmrun_Interfaces) CloudInit() pb_cloudinit.CloudInitServiceClient {
	return k.cloudinitClient
}

func (k *Kvmrun_Interfaces) Hardware() pb_hardware.HardwareServiceClient {
	return k.hardwareClient
}

func (s *Server) KvmrunGRPC(host string, fn func(*Kvmrun_Interfaces) error) error {
	hostport := host + ":9393"

	conn, err := grpcclient.NewSecureConnection(hostport, s.AppConf.TLSConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	K := Kvmrun_Interfaces{
		machinesClient:  pb_machines.NewMachineServiceClient(conn),
		systemClient:    pb_system.NewSystemServiceClient(conn),
		networkClient:   pb_network.NewNetworkServiceClient(conn),
		tasksClient:     pb_tasks.NewTaskServiceClient(conn),
		cloudinitClient: pb_cloudinit.NewCloudInitServiceClient(conn),
		hardwareClient:  pb_hardware.NewHardwareServiceClient(conn),
	}

	return fn(&K)
}
