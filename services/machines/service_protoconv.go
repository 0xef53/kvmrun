package machines

import (
	"strings"
	"time"

	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server/machine"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"

	qmp "github.com/0xef53/go-qmp/v2"
)

func machineToProto(vm *kvmrun.Machine, vmstate kvmrun.InstanceState, t time.Duration) *pb_types.Machine {
	m := pb_types.Machine{
		Name:       vm.Name,
		Persistent: true,
		State:      pb_types.MachineState(vmstate),
		LifeTime:   uint64(t),
	}

	conv := func(vmi kvmrun.Instance) *pb_types.MachineOpts {
		opts := pb_types.MachineOpts{
			MachineType: vmi.MachineTypeGet().String(),
			Memory: &pb_types.MachineOpts_Memory{
				Actual: uint32(vmi.MemoryGetActual()),
				Total:  uint32(vmi.MemoryGetTotal()),
			},
			CPU: &pb_types.MachineOpts_CPU{
				Actual:  uint32(vmi.CPUGetActual()),
				Total:   uint32(vmi.CPUGetTotal()),
				Sockets: uint32(vmi.CPUGetSockets()),
				Model:   vmi.CPUGetModel(),
				Quota:   uint32(vmi.CPUGetQuota()),
			},
		}

		for _, d := range vmi.InputDeviceGetList() {
			opts.Inputs = append(opts.Inputs, &pb_types.MachineOpts_InputDevice{
				Type: d.Type,
			})
		}

		for _, d := range vmi.CdromGetList() {
			opts.Cdrom = append(opts.Cdrom, &pb_types.MachineOpts_Cdrom{
				Name:      d.Name,
				Media:     d.Media,
				Driver:    d.Driver().String(),
				Readonly:  d.Readonly,
				Bootindex: uint32(d.Bootindex),
				Addr:      d.QemuAddr,
			})
		}

		for _, d := range vmi.DiskGetList() {
			opts.Storage = append(opts.Storage, &pb_types.MachineOpts_Disk{
				Path:      d.Path,
				Driver:    d.Driver().String(),
				IopsRd:    uint32(d.IopsRd),
				IopsWr:    uint32(d.IopsWr),
				Bootindex: uint32(d.Bootindex),
				Addr:      d.QemuAddr,
			})
		}

		for _, n := range vmi.NetIfaceGetList() {
			opts.Network = append(opts.Network, &pb_types.MachineOpts_NetIface{
				Ifname: n.Ifname,
				Driver: n.Driver().String(),
				HwAddr: n.HwAddr,
				Queues: uint32(n.Queues),
				Ifup:   n.Ifup,
				Ifdown: n.Ifdown,
				Addr:   n.QemuAddr,
			})
		}

		if image := vmi.KernelGetImage(); len(image) > 0 {
			opts.Kernel = &pb_types.MachineOpts_Kernel{
				Image:   image,
				Initrd:  vmi.KernelGetInitrd(),
				Modiso:  vmi.KernelGetModiso(),
				Cmdline: vmi.KernelGetCmdline(),
			}
		}

		if fwimage := vmi.FirmwareGetImage(); len(fwimage) != 0 {
			opts.Firmware = &pb_types.MachineOpts_Firmware{
				Image: fwimage,
			}
			if fwflash := vmi.FirmwareGetFlash(); fwflash != nil {
				opts.Firmware.Flash = fwflash.Path
			}
		}

		vsock := vmi.VSockDeviceGet()

		if vsock != nil {
			opts.VsockDevice = &pb_types.MachineOpts_ChannelVSock{
				ContextID: vsock.ContextID,
				Addr:      vsock.QemuAddr,
			}
		}

		if cidrive := vmi.CloudInitGetDrive(); cidrive != nil {
			opts.CloudInitDrive = &pb_types.MachineOpts_CloudInit{
				Path:   cidrive.Media,
				Driver: cidrive.Driver().String(),
			}
		}

		for _, d := range vmi.HostDeviceGetList() {
			opts.HostPCI = append(opts.HostPCI, &pb_types.MachineOpts_HostDevice{
				PCIAddr:       d.BackendAddr.String(),
				Multifunction: d.Multifunction,
				PrimaryGPU:    d.PrimaryGPU,
			})
		}

		return &opts
	}

	if vm.C != nil {
		m.Config = conv(vm.C)
	}

	if vm.R != nil {
		m.Runtime = conv(vm.R)
		m.PID = uint32(vm.R.PID())
	}

	return &m
}

func propertiesFromMachineOpts(proto *pb_types.MachineOpts) *kvmrun.InstanceProperties {
	opts := kvmrun.InstanceProperties{
		MachineType: proto.MachineType,
	}

	if proto.Firmware != nil {
		opts.Firmware = &kvmrun.Firmware{
			FirmwareProperties: kvmrun.FirmwareProperties{
				Image: proto.Firmware.Image,
				Flash: proto.Firmware.Flash,
			},
		}
	}

	if proto.Memory != nil {
		opts.Memory.Actual = int(proto.Memory.Actual)
		opts.Memory.Total = int(proto.Memory.Total)
	}

	if proto.CPU != nil {
		opts.CPU.Actual = int(proto.CPU.Actual)
		opts.CPU.Total = int(proto.CPU.Total)
		opts.CPU.Sockets = int(proto.CPU.Sockets)
		opts.CPU.Model = proto.CPU.Model
		opts.CPU.Quota = int(proto.CPU.Quota)
	}

	for _, v := range proto.Inputs {
		opts.InputDevices.Append(&kvmrun.InputDevice{
			InputDeviceProperties: kvmrun.InputDeviceProperties{
				Type: v.Type,
			},
		})
	}

	for _, v := range proto.Cdrom {
		opts.Cdroms.Append(&kvmrun.Cdrom{
			CdromProperties: kvmrun.CdromProperties{
				Name:     v.Name,
				Media:    v.Media,
				Driver:   v.Driver,
				Readonly: v.Readonly,
			},
		})
	}

	for _, v := range proto.Storage {
		opts.Disks.Append(&kvmrun.Disk{
			DiskProperties: kvmrun.DiskProperties{
				Path:   v.Path,
				Driver: v.Driver,
				IopsRd: int(v.IopsRd),
				IopsWr: int(v.IopsWr),
			},
		})
	}

	for _, v := range proto.Network {
		opts.NetIfaces.Append(&kvmrun.NetIface{
			NetIfaceProperties: kvmrun.NetIfaceProperties{
				Ifname: v.Ifname,
				Driver: v.Driver,
				HwAddr: v.HwAddr,
				Queues: int(v.Queues),
				Ifup:   v.Ifup,
				Ifdown: v.Ifdown,
			},
		})
	}

	for _, v := range proto.HostPCI {
		opts.HostDevices.Append(&kvmrun.HostDevice{
			HostDeviceProperties: kvmrun.HostDeviceProperties{
				PCIAddr:       v.PCIAddr,
				Multifunction: v.Multifunction,
				PrimaryGPU:    v.PrimaryGPU,
			},
		})
	}

	if proto.VsockDevice != nil {
		opts.VSockDevice = &kvmrun.ChannelVSock{
			ChannelVSockProperties: kvmrun.ChannelVSockProperties{
				ContextID: proto.VsockDevice.ContextID,
			},
		}
	}

	if proto.CloudInitDrive != nil {
		opts.CloudInitDrive = &kvmrun.CloudInitDrive{
			CloudInitDriveProperties: kvmrun.CloudInitDriveProperties{
				Media:  proto.CloudInitDrive.Path,
				Driver: proto.CloudInitDrive.Driver,
			},
		}
	}

	if proto.Kernel != nil {
		opts.Kernel.Image = proto.Kernel.Image
		opts.Kernel.Initrd = proto.Kernel.Initrd
		opts.Kernel.Modiso = proto.Kernel.Modiso
		opts.Kernel.Cmdline = proto.Kernel.Cmdline
	}

	return &opts
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

func optsFromNetIfaceAttachRequest(req *pb.NetIfaceAttachRequest) *kvmrun.NetIfaceProperties {
	return &kvmrun.NetIfaceProperties{
		Ifname: req.Ifname,
		Driver: strings.ReplaceAll(strings.ToLower(req.Driver.String()), "_", "-"),
		HwAddr: req.HwAddr,
		Queues: int(req.Queues),
		Ifup:   req.IfupScript,
		Ifdown: req.IfdownScript,
	}
}

func optsFromHostDeviceAttachRequest(req *pb.HostDeviceAttachRequest) *kvmrun.HostDeviceProperties {
	return &kvmrun.HostDeviceProperties{
		PCIAddr:       req.PCIAddr,
		PrimaryGPU:    req.PrimaryGPU,
		Multifunction: req.Multifunction,
	}
}

func optsFromInputDeviceAttachRequest(req *pb.InputDeviceAttachRequest) *kvmrun.InputDeviceProperties {
	return &kvmrun.InputDeviceProperties{
		Type: strings.ReplaceAll(strings.ToLower(req.Type.String()), "_", "-"),
	}
}

func optsFromDiskAttachRequest(req *pb.DiskAttachRequest) *kvmrun.DiskProperties {
	return &kvmrun.DiskProperties{
		Path:      req.DiskPath,
		Driver:    strings.ReplaceAll(strings.ToLower(req.Driver.String()), "_", "-"),
		IopsRd:    int(req.IopsRd),
		IopsWr:    int(req.IopsWr),
		Bootindex: int(req.Bootindex),
	}
}

func optsFromCdromAttachRequest(req *pb.CdromAttachRequest) *kvmrun.CdromProperties {
	return &kvmrun.CdromProperties{
		Name:      req.DeviceName,
		Media:     req.DeviceMedia,
		Driver:    strings.ReplaceAll(strings.ToLower(req.Driver.String()), "_", "-"),
		Bootindex: int(req.Bootindex),
		Readonly:  req.Readonly,
	}
}

func optsFromCloudInitDriveAttachRequest(req *pb.CloudInitDriveAttachRequest) *kvmrun.CloudInitDriveProperties {
	driver := strings.ReplaceAll(strings.ToLower(req.Driver.String()), "_", "-")

	return &kvmrun.CloudInitDriveProperties{
		Media:  req.Media,
		Driver: strings.TrimPrefix(driver, "ci-"),
	}
}

func optsFromFirmwareSetRequest(req *pb.FirmwareSetRequest) *kvmrun.FirmwareProperties {
	return &kvmrun.FirmwareProperties{
		Image: req.Image,
		Flash: req.Flash,
	}
}

func optsFromExternalKernelSetRequest(req *pb.ExternalKernelSetRequest) *kvmrun.ExtKernelProperties {
	return &kvmrun.ExtKernelProperties{
		Image:   req.Image,
		Initrd:  req.Initrd,
		Cmdline: req.Cmdline,
		Modiso:  req.Modiso,
	}
}

func optsFromStartDiskBackupRequest(req *pb.StartDiskBackupRequest) *machine.DiskBackupOptions {
	return &machine.DiskBackupOptions{
		DiskName:    req.DiskName,
		Target:      req.Target,
		Incremental: req.Incremental,
		ClearBitmap: req.ClearBitmap,
	}
}

func optsFromStartMigrationRequest(req *pb.StartMigrationRequest) *machine.MigrationOptions {
	opts := machine.MigrationOptions{
		Disks:       req.Disks,
		CreateDisks: req.CreateDisks,
		RemoveAfter: req.RemoveAfter,
	}

	if req.Overrides != nil {
		opts.Overrides = machine.MigrationOverrides{
			Name:      req.Overrides.Name,
			Disks:     req.Overrides.Disks,
			NetIfaces: req.Overrides.NetIfaces,
		}
	}

	return &opts
}

func vncRequisitesToProto(requisites *machine.VNCRequisites) *pb_types.VNCRequisites {
	return &pb_types.VNCRequisites{
		Password: requisites.Password,
		Display:  uint32(requisites.Display),
		Port:     uint32(requisites.Port),
		WSPort:   uint32(requisites.WSPort),
	}
}
