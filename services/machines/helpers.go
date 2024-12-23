package machines

import (
	"time"

	qmp "github.com/0xef53/go-qmp/v2"
	pb_types "github.com/0xef53/kvmrun/api/types"
	"github.com/0xef53/kvmrun/kvmrun"
)

func machineToProto(vm *kvmrun.Machine, vmstate kvmrun.InstanceState, t time.Duration) *pb_types.Machine {
	m := pb_types.Machine{
		Name:       vm.Name,
		Persistent: true,
		State:      pb_types.MachineState(vmstate),
		LifeTime:   int64(t),
	}

	conv := func(vmi kvmrun.Instance) *pb_types.MachineOpts {
		opts := pb_types.MachineOpts{
			MachineType: vmi.GetMachineType().String(),
			Memory: &pb_types.MachineOpts_Memory{
				Actual: int64(vmi.GetActualMem()),
				Total:  int64(vmi.GetTotalMem()),
			},
			CPU: &pb_types.MachineOpts_CPU{
				Actual:  int64(vmi.GetActualCPUs()),
				Total:   int64(vmi.GetTotalCPUs()),
				Sockets: int32(vmi.GetCPUSockets()),
				Model:   vmi.GetCPUModel(),
				Quota:   int64(vmi.GetCPUQuota()),
			},
		}

		for _, d := range vmi.GetInputDevices() {
			opts.Inputs = append(opts.Inputs, &pb_types.MachineOpts_InputDevice{
				Type: d.Type,
			})
		}

		for _, d := range vmi.GetCdroms() {
			opts.Cdrom = append(opts.Cdrom, &pb_types.MachineOpts_Cdrom{
				Name:     d.Name,
				Media:    d.Media,
				Driver:   d.Driver,
				ReadOnly: d.ReadOnly,
				Addr:     d.Addr,
			})
		}

		for _, d := range vmi.GetDisks() {
			opts.Storage = append(opts.Storage, &pb_types.MachineOpts_Disk{
				Path:   d.Path,
				Driver: d.Driver,
				IopsRd: int64(d.IopsRd),
				IopsWr: int64(d.IopsWr),
				Addr:   d.Addr,
			})
		}

		for _, p := range vmi.GetProxyServers() {
			opts.Proxy = append(opts.Proxy, &pb_types.MachineOpts_BackendProxy{
				Path:    p.Path,
				Command: p.Command,
				Envs:    p.Envs,
			})
		}

		for _, n := range vmi.GetNetIfaces() {
			opts.Network = append(opts.Network, &pb_types.MachineOpts_NetIface{
				Ifname: n.Ifname,
				Driver: n.Driver,
				HwAddr: n.HwAddr,
				Queues: uint32(n.Queues),
				Ifup:   n.Ifup,
				Ifdown: n.Ifdown,
				Addr:   n.Addr,
			})
		}

		if len(vmi.GetKernelImage()) != 0 {
			opts.Kernel = &pb_types.MachineOpts_Kernel{
				Image:   vmi.GetKernelImage(),
				Initrd:  vmi.GetKernelInitrd(),
				Modiso:  vmi.GetKernelModiso(),
				Cmdline: vmi.GetKernelCmdline(),
			}
		}

		if fwimage := vmi.GetFirmwareImage(); len(fwimage) != 0 {
			opts.Firmware = &pb_types.MachineOpts_Firmware{
				Image: fwimage,
			}
			if fwflash := vmi.GetFirmwareFlash(); fwflash != nil {
				opts.Firmware.Flash = fwflash.Path
			}
		}

		vsock := vmi.GetVSockDevice()

		if vsock != nil {
			opts.VSockDev = &pb_types.MachineOpts_VirtioVSock{
				Auto:      vsock.Auto,
				ContextID: vsock.ContextID,
				Addr:      vsock.Addr,
			}
		}

		if cidrive := vmi.GetCloudInitDrive(); cidrive != nil {
			opts.CIDrive = &pb_types.MachineOpts_CloudInit{
				Path:   cidrive.Media,
				Driver: cidrive.Driver,
			}
		}

		for _, d := range vmi.GetHostPCIDevices() {
			opts.HostPCIDevices = append(opts.HostPCIDevices, &pb_types.MachineOpts_HostPCI{
				Addr:          d.BackendAddr.String(),
				Multifunction: d.Multifunction,
				PrimaryGPU:    d.PrimaryGPU,
			})
		}

		return &opts
	}

	m.Config = conv(vm.C)

	if vm.R != nil {
		m.Runtime = conv(vm.R)
		m.Pid = int32(vm.R.Pid())
	}

	return &m
}

func eventsToProto(ee []qmp.Event) []*pb_types.MachineEvent {
	events := make([]*pb_types.MachineEvent, 0, len(ee))

	for _, e := range ee {
		events = append(events, &pb_types.MachineEvent{
			Type: e.Type,
			Data: e.Data,
			Timestamp: &pb_types.MachineEvent_Timestamp{
				Seconds:      e.Timestamp.Seconds,
				Microseconds: e.Timestamp.Microseconds,
			},
		})
	}

	return events
}

func stringSliceContains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}

	return false
}
