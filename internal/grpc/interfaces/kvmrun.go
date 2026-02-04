package interfaces

import (
	pb_cloudinit "github.com/0xef53/kvmrun/api/services/cloudinit/v2"
	pb_hardware "github.com/0xef53/kvmrun/api/services/hardware/v2"
	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_network "github.com/0xef53/kvmrun/api/services/network/v2"
	pb_system "github.com/0xef53/kvmrun/api/services/system/v2"
	pb_tasks "github.com/0xef53/kvmrun/api/services/tasks/v2"

	"google.golang.org/grpc"
)

type Kvmrun struct {
	Client_Machines  pb_machines.MachineServiceClient
	Client_System    pb_system.SystemServiceClient
	Client_Network   pb_network.NetworkServiceClient
	Client_Tasks     pb_tasks.TaskServiceClient
	Client_CloudInit pb_cloudinit.CloudInitServiceClient
	Client_Hardware  pb_hardware.HardwareServiceClient
}

func NewKvmrunInterface(conn *grpc.ClientConn) *Kvmrun {
	return &Kvmrun{
		Client_Machines:  pb_machines.NewMachineServiceClient(conn),
		Client_System:    pb_system.NewSystemServiceClient(conn),
		Client_Network:   pb_network.NewNetworkServiceClient(conn),
		Client_Tasks:     pb_tasks.NewTaskServiceClient(conn),
		Client_CloudInit: pb_cloudinit.NewCloudInitServiceClient(conn),
		Client_Hardware:  pb_hardware.NewHardwareServiceClient(conn),
	}
}

func (k *Kvmrun) Machines() pb_machines.MachineServiceClient {
	return k.Client_Machines
}

func (k *Kvmrun) System() pb_system.SystemServiceClient {
	return k.Client_System
}

func (k *Kvmrun) Network() pb_network.NetworkServiceClient {
	return k.Client_Network
}

func (k *Kvmrun) Tasks() pb_tasks.TaskServiceClient {
	return k.Client_Tasks
}

func (k *Kvmrun) CloudInit() pb_cloudinit.CloudInitServiceClient {
	return k.Client_CloudInit
}

func (k *Kvmrun) Hardware() pb_hardware.HardwareServiceClient {
	return k.Client_Hardware
}
