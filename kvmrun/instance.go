package kvmrun

import (
	"strings"

	"github.com/0xef53/kvmrun/internal/version"
)

type Instance interface {
	Name() string
	Uid() int
	Pid() int

	GetQemuVersion() *version.Version

	Status() (InstanceState, error)
	GetMachineType() *QemuMachine
	SetMachineType(string) error

	IsIncoming() bool

	GetFirmwareImage() string
	GetFirmwareFlash() *Disk
	SetFirmwareImage(string) error
	SetFirmwareFlash(string) error
	RemoveFirmwareConf() error

	Save() error

	GetActualMem() int
	GetTotalMem() int
	SetTotalMem(int) error
	SetActualMem(int) error

	GetActualCPUs() int
	GetTotalCPUs() int
	GetCPUSockets() int
	GetCPUQuota() int
	GetCPUModel() string
	SetActualCPUs(int) error
	SetTotalCPUs(int) error
	SetCPUSockets(int) error
	SetCPUQuota(int) error
	SetCPUModel(string) error

	GetHostPCIDevices() HostPCIPool
	AppendHostPCI(HostPCI) error
	RemoveHostPCI(string) error
	SetHostPCIMultifunctionOption(string, bool) error
	SetHostPCIPrimaryGPUOption(string, bool) error

	GetInputDevices() InputPool
	AppendInputDevice(InputDevice) error
	RemoveInputDevice(string) error

	GetCdroms() CDPool
	AppendCdrom(Cdrom) error
	InsertCdrom(Cdrom, int) error
	RemoveCdrom(string) error
	GetCdromMedia(string) (string, error)
	ChangeCdromMedia(string, string) error

	GetDisks() DiskPool
	ResizeQemuBlockdev(string) error
	AppendDisk(Disk) error
	InsertDisk(Disk, int) error
	RemoveDisk(string) error
	SetDiskReadIops(string, int) error
	SetDiskWriteIops(string, int) error
	RemoveDiskBitmap(string) error

	GetProxyServers() ProxyPool
	AppendProxy(Proxy) error
	RemoveProxy(string) error

	GetNetIfaces() NetifPool
	AppendNetIface(NetIface) error
	RemoveNetIface(string) error
	SetNetIfaceQueues(string, int) error
	SetNetIfaceUpScript(string, string) error
	SetNetIfaceDownScript(string, string) error
	SetNetIfaceLinkUp(string) error
	SetNetIfaceLinkDown(string) error

	GetVSockDevice() *VirtioVSock
	AppendVSockDevice(uint32) error
	RemoveVSockDevice() error

	GetCloudInitDrive() *CloudInitDrive
	SetCloudInitMedia(string) error
	SetCloudInitDriver(string) error
	RemoveCloudInitConf() error

	GetKernelImage() string
	GetKernelCmdline() string
	GetKernelInitrd() string
	GetKernelModiso() string
	RemoveKernelConf() error
	SetKernelImage(string) error
	SetKernelCmdline(string) error
	SetKernelInitrd(string) error
	SetKernelModiso(string) error

	SetVNCPassword(string) error
}

type InstanceProperties struct {
	name string `json:"-"`
	uid  int    `json:"-"`

	MachineType string       `json:"machine_type,omitempty"`
	Firmware    QemuFirmware `json:"firmware,omitempty"`

	Mem            Memory          `json:"memory"`
	CPU            Processor       `json:"cpu"`
	Inputs         InputPool       `json:"inputs"`
	Cdroms         CDPool          `json:"cdrom"`
	Disks          DiskPool        `json:"storage"`
	Proxy          ProxyPool       `json:"proxy,omitempty"`
	NetIfaces      NetifPool       `json:"network"`
	VSockDevice    *VirtioVSock    `json:"vsock_device,omitempty"`
	CIDrive        *CloudInitDrive `json:"cloudinit_drive,omitempty"`
	Kernel         ExtKernel       `json:"kernel"`
	HostPCIDevices HostPCIPool     `json:"hostpci"`
}

func (p InstanceProperties) Name() string {
	return p.name
}

func (p InstanceProperties) Uid() int {
	return p.uid
}

func (p InstanceProperties) GetQemuVersion() *version.Version {
	return version.MustParse(0)
}

func (p InstanceProperties) GetMachineType() *QemuMachine {
	var chipset string

	ff := strings.Split(p.MachineType, "-")

	switch len(ff) {
	case 3:
		chipset = ff[1]
	case 1:
		chipset = ff[0]
	}

	switch chipset {
	case "", "pc", "i440fx":
		return &QemuMachine{name: p.MachineType, Chipset: QEMU_CHIPSET_I440FX}
	case "q35":
		return &QemuMachine{name: p.MachineType, Chipset: QEMU_CHIPSET_Q35}
	case "microvm":
		return &QemuMachine{name: p.MachineType, Chipset: QEMU_CHIPSET_MICROVM}
	}

	return &QemuMachine{name: p.MachineType, Chipset: QEMU_CHIPSET_UNKNOWN}
}

func (p InstanceProperties) GetFirmwareImage() string {
	return p.Firmware.Image
}

func (p InstanceProperties) GetFirmwareFlash() *Disk {
	return p.Firmware.flashDisk
}

func (p InstanceProperties) GetActualCPUs() int {
	return p.CPU.Actual
}

func (p InstanceProperties) GetTotalCPUs() int {
	return p.CPU.Total
}

func (p InstanceProperties) GetCPUSockets() int {
	return p.CPU.Sockets
}

func (p InstanceProperties) GetCPUModel() string {
	return p.CPU.Model
}

func (p InstanceProperties) GetCPUQuota() int {
	return p.CPU.Quota
}

func (p InstanceProperties) GetActualMem() int {
	return p.Mem.Actual
}

func (p InstanceProperties) GetTotalMem() int {
	return p.Mem.Total
}

func (p InstanceProperties) GetHostPCIDevices() HostPCIPool {
	// Currently deep copy is not needed
	pool := make(HostPCIPool, len(p.HostPCIDevices))

	copy(pool, p.HostPCIDevices)

	return pool
}

func (p InstanceProperties) GetInputDevices() InputPool {
	// Currently deep copy is not needed
	pool := make(InputPool, len(p.Inputs))

	copy(pool, p.Inputs)

	return pool
}

func (p InstanceProperties) GetCdroms() CDPool {
	// Currently deep copy is not needed
	pool := make(CDPool, len(p.Cdroms))

	copy(pool, p.Cdroms)

	return pool
}

func (p InstanceProperties) GetCdromMedia(name string) (string, error) {
	d := p.Cdroms.Get(name)
	if d == nil {
		return "", &NotConnectedError{"instance", name}
	}

	return d.Media, nil
}

func (p InstanceProperties) GetDisks() DiskPool {
	// Currently deep copy is not needed
	pool := make(DiskPool, len(p.Disks))

	copy(pool, p.Disks)

	return pool
}

func (p InstanceProperties) GetProxyServers() ProxyPool {
	// Currently deep copy is not needed
	pool := make(ProxyPool, len(p.Proxy))

	copy(pool, p.Proxy)

	return pool
}

func (p InstanceProperties) GetNetIfaces() NetifPool {
	// Currently deep copy is not needed
	pool := make(NetifPool, len(p.NetIfaces))

	copy(pool, p.NetIfaces)

	return pool
}

func (p InstanceProperties) GetVSockDevice() *VirtioVSock {
	return p.VSockDevice
}

func (p InstanceProperties) GetCloudInitDrive() *CloudInitDrive {
	if p.CIDrive == nil {
		return nil
	}

	return &CloudInitDrive{
		Media:   p.CIDrive.Media,
		Driver:  p.CIDrive.Driver,
		Backend: p.CIDrive.Backend.Copy(),
	}
}

func (p InstanceProperties) GetKernelImage() string {
	return p.Kernel.Image
}

func (p InstanceProperties) GetKernelCmdline() string {
	return p.Kernel.Cmdline
}

func (p InstanceProperties) GetKernelInitrd() string {
	return p.Kernel.Initrd
}

func (p InstanceProperties) GetKernelModiso() string {
	return p.Kernel.Modiso
}
