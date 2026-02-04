package kvmrun

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/0xef53/kvmrun/internal/version"
)

var machineNameRe = regexp.MustCompile(`^[0-9A-Za-z_]{3,16}$`)

func ValidateMachineName(name string) error {
	if machineNameRe.MatchString(name) {
		return nil
	}

	return fmt.Errorf("invalid machine name: only [0-9A-Za-z_] are allowed, min length is 3 and max length is 16")
}

type InstanceProperties struct {
	name string `json:"-"`
	uid  int    `json:"-"`

	MachineType string    `json:"machine_type,omitempty"`
	Firmware    *Firmware `json:"firmware,omitempty"`

	Memory         Memory          `json:"memory"`
	CPU            VirtCPU         `json:"cpu"`
	InputDevices   InputDevicePool `json:"inputs"`
	Cdroms         CdromPool       `json:"cdrom"`
	Disks          DiskPool        `json:"storage"`
	NetIfaces      NetIfacePool    `json:"network"`
	VSockDevice    *ChannelVSock   `json:"vsock_device,omitempty"`
	CloudInitDrive *CloudInitDrive `json:"cloudinit_drive,omitempty"`
	Kernel         ExtKernel       `json:"kernel"`
	HostDevices    HostDevicePool  `json:"hostpci"`
}

func (p *InstanceProperties) Validate(strict bool) error {
	p.MachineType = strings.TrimSpace(p.MachineType)

	if len(p.MachineType) > 0 && strict {
		if _, err := ParseMachineType(p.MachineType); err != nil {
			return err
		}
	}

	if p.Firmware != nil {
		p.Firmware.Image = strings.TrimSpace(p.Firmware.Image)
		p.Firmware.Flash = strings.TrimSpace(p.Firmware.Flash)
	}

	if err := p.Memory.Validate(strict); err != nil {
		return err
	}

	if err := p.CPU.Validate(strict); err != nil {
		return err
	}

	return nil
}

func (p *InstanceProperties) Name() string {
	return p.name
}

func (p *InstanceProperties) UID() int {
	return p.uid
}

func (p *InstanceProperties) PID() int {
	return 0
}

func (p *InstanceProperties) QemuVersion() *version.Version {
	return version.MustParse(0)
}

func (p *InstanceProperties) MachineTypeGet() *MachineType {
	t, err := ParseMachineType(p.MachineType)
	if err != nil {
		// err is "unsupported error"
		return &MachineType{name: p.MachineType, Chipset: QEMU_CHIPSET_UNKNOWN}
	}

	return t
}

func (p *InstanceProperties) FirmwareGet() *Firmware {
	if p.Firmware == nil {
		return nil
	}

	return p.Firmware.Copy()
}

func (p *InstanceProperties) FirmwareGetImage() string {
	if p.Firmware == nil {
		return ""
	}

	return p.Firmware.Image
}

func (p *InstanceProperties) FirmwareGetFlash() *Disk {
	if p.Firmware != nil && p.Firmware.flashDisk != nil {
		return p.Firmware.flashDisk.Copy()
	}

	return nil
}

func (p *InstanceProperties) MemoryGetActual() int {
	return p.Memory.Actual
}

func (p *InstanceProperties) MemoryGetTotal() int {
	return p.Memory.Total
}

func (p *InstanceProperties) CPUGetActual() int {
	return p.CPU.Actual
}

func (p *InstanceProperties) CPUGetTotal() int {
	return p.CPU.Total
}

func (p *InstanceProperties) CPUGetSockets() int {
	return p.CPU.Sockets
}

func (p *InstanceProperties) CPUGetModel() string {
	return p.CPU.Model
}

func (p *InstanceProperties) CPUGetQuota() int {
	return p.CPU.Quota
}

func (p *InstanceProperties) InputDeviceGet(devtype string) *InputDevice {
	dev := p.InputDevices.Get(devtype)

	if dev == nil {
		return nil
	}

	return dev.Copy()
}

func (p *InstanceProperties) InputDeviceGetList(devtypes ...string) []*InputDevice {
	values := make([]*InputDevice, 0, p.InputDevices.Len())

	for _, dev := range p.InputDevices.Values(devtypes...) {
		values = append(values, dev.Copy())
	}

	return values
}

func (p *InstanceProperties) CdromGet(devname string) *Cdrom {
	cd := p.Cdroms.Get(devname)

	if cd == nil {
		return nil
	}

	return cd.Copy()
}

func (p *InstanceProperties) CdromGetList(devnames ...string) []*Cdrom {
	values := make([]*Cdrom, 0, p.Cdroms.Len())

	for _, cd := range p.Cdroms.Values(devnames...) {
		values = append(values, cd.Copy())
	}

	return values
}

func (p *InstanceProperties) CdromGetMedia(devname string) (string, error) {
	cd := p.Cdroms.Get(devname)

	if cd == nil {
		return "", &NotConnectedError{"instance", devname}
	}

	return cd.Media, nil
}

func (p *InstanceProperties) DiskGet(diskname string) *Disk {
	d := p.Disks.Get(diskname)

	if d == nil {
		return nil
	}

	return d.Copy()
}

func (p *InstanceProperties) DiskGetList(disknames ...string) []*Disk {
	values := make([]*Disk, 0, p.Disks.Len())

	for _, d := range p.Disks.Values(disknames...) {
		values = append(values, d.Copy())
	}

	return values
}

func (p *InstanceProperties) NetIfaceGet(ifname string) *NetIface {
	n := p.NetIfaces.Get(ifname)

	if n == nil {
		return nil
	}

	return n.Copy()
}

func (p *InstanceProperties) NetIfaceGetList(ifnames ...string) []*NetIface {
	values := make([]*NetIface, 0, p.NetIfaces.Len())

	for _, n := range p.NetIfaces.Values(ifnames...) {
		values = append(values, n.Copy())
	}

	return values
}

func (p *InstanceProperties) VSockDeviceGet() *ChannelVSock {
	if p.VSockDevice == nil {
		return nil
	}

	return p.VSockDevice.Copy()
}

func (p *InstanceProperties) CloudInitGetDrive() *CloudInitDrive {
	if p.CloudInitDrive == nil {
		return nil
	}

	return p.CloudInitDrive.Copy()
}

func (p *InstanceProperties) KernelGetImage() string {
	return p.Kernel.Image
}

func (p *InstanceProperties) KernelGetCmdline() string {
	return p.Kernel.Cmdline
}

func (p *InstanceProperties) KernelGetInitrd() string {
	return p.Kernel.Initrd
}

func (p *InstanceProperties) KernelGetModiso() string {
	return p.Kernel.Modiso
}

func (p *InstanceProperties) HostDeviceGet(hexaddr string) *HostDevice {
	dev := p.HostDevices.Get(hexaddr)

	if dev == nil {
		return nil
	}

	return dev.Copy()
}

func (p *InstanceProperties) HostDeviceGetList(hexaddrs ...string) []*HostDevice {
	values := make([]*HostDevice, 0, p.HostDevices.Len())

	for _, dev := range p.HostDevices.Values(hexaddrs...) {
		values = append(values, dev.Copy())
	}

	return values
}
