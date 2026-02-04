package kvmrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun/backend"
	"github.com/0xef53/kvmrun/kvmrun/backend/file"
	"github.com/0xef53/kvmrun/kvmrun/internal/pool"
)

// InstanceConf represents a virtual machine configuration
// that is used to prepare a QEMU command line.
type InstanceConf struct {
	*InstanceProperties

	confname string `json:"-"`
}

func newInstanceConf(vmname string) *InstanceConf {
	vmc := InstanceConf{
		InstanceProperties: &InstanceProperties{
			name: vmname,
		},
		confname: "config",
	}

	vmc.Memory.Total = 128
	vmc.Memory.Actual = 128
	vmc.CPU.Total = 1
	vmc.CPU.Actual = 1

	return &vmc
}

func NewInstanceConf(vmname string) Instance {
	return Instance(newInstanceConf(vmname))
}

func GetInstanceConf(vmname string) (Instance, error) {
	vmname = strings.TrimSpace(vmname)

	if len(vmname) == 0 {
		return nil, fmt.Errorf("empty machine name")
	}

	vmc := newInstanceConf(vmname)

	if b, err := os.ReadFile(vmc.config()); err == nil {
		if err := json.Unmarshal(b, vmc); err != nil {
			return nil, err
		}
	} else {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, vmname)
		}
		return nil, err
	}

	vmc.MachineType = strings.TrimSpace(strings.ToLower(vmc.MachineType))

	// If a path to persistent flash is set, flashDisk must not be nil.
	if vmc.Firmware != nil && len(vmc.Firmware.Flash) > 0 && vmc.Firmware.flashDisk == nil {
		return nil, &backend.UnknownBackendError{Path: vmc.Firmware.Flash}
	}

	// Each cdrom device must have a non-nil backend.
	for _, cd := range vmc.Cdroms.Values() {
		if len(cd.Media) > 0 && cd.MediaBackend == nil {
			return nil, &backend.UnknownBackendError{Path: cd.Media}
		}
	}

	// Each disk device must have a non-nil backend.
	for _, d := range vmc.Disks.Values() {
		if d.Backend == nil {
			return nil, &backend.UnknownBackendError{Path: d.Path}
		}
	}

	// CloudInit drive must have a non-nil backend.
	if vmc.CloudInitDrive != nil && vmc.CloudInitDrive.Backend == nil {
		return nil, &backend.UnknownBackendError{Path: vmc.CloudInitDrive.Media}
	}

	// Each host-PCI device must have a non-nil backend address.
	for _, dev := range vmc.HostDevices.Values() {
		if dev.BackendAddr == nil {
			return nil, fmt.Errorf("invalid host-pci addr: %s", dev.PCIAddr)
		}
	}

	vmuser, err := user.Lookup(vmname)
	if err != nil {
		return nil, err
	}

	if uid, err := strconv.Atoi(vmuser.Uid); err == nil {
		vmc.uid = uid
	} else {
		return nil, err
	}

	return Instance(vmc), nil
}

func (c InstanceConf) config() string {
	return filepath.Join(CONFDIR, c.name, c.confname)
}

func (c InstanceConf) IsIncoming() bool {
	return c.confname == "incoming_config"
}

func (c InstanceConf) Save() error {
	if err := ValidateMachineName(c.Name()); err != nil {
		return err
	}

	b, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.config(), append(b, '\n'), 0644)
}

func (c InstanceConf) SaveStartupConfig() error {
	if err := ValidateMachineName(c.Name()); err != nil {
		return err
	}

	startupConfig := filepath.Join(CHROOTDIR, c.name, "run/startup_config")

	b, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(startupConfig, append(b, '\n'), 0644)
}

func (c InstanceConf) Status() (InstanceState, error) {
	return StateInactive, nil
}

func (c *InstanceConf) MachineTypeSet(typename string) error {
	t, err := ParseMachineType(typename)
	if err != nil {
		return err
	}

	c.MachineType = t.name

	return nil
}

func (c *InstanceConf) FirmwareSetImage(image string) error {
	if c.Firmware == nil {
		if fw, err := NewFirmware(image, ""); err == nil {
			c.Firmware = fw
		} else {
			return err
		}
	}

	return c.Firmware.SetImage(image)
}

func (c *InstanceConf) FirmwareSetFlash(flash string) error {
	if c.Firmware == nil {
		return fmt.Errorf("firmware image must be set before this operation")
	}

	return c.Firmware.SetFlash(flash)
}

func (c *InstanceConf) FirmwareRemoveConf() error {
	c.Firmware = nil

	return nil
}

func (c *InstanceConf) MemorySetActual(value int) error {
	return c.Memory.SetActual(value)
}

func (c *InstanceConf) MemorySetTotal(value int) error {
	return c.Memory.SetTotal(value)
}

func (c *InstanceConf) CPUSetActual(value int) error {
	return c.CPU.SetActual(value)
}

func (c *InstanceConf) CPUSetTotal(value int) error {
	return c.CPU.SetTotal(value)
}

func (c *InstanceConf) CPUSetSockets(value int) error {
	return c.CPU.SetSockets(value)
}

func (c *InstanceConf) CPUSetModel(value string) error {
	return c.CPU.SetModel(value)
}

func (c *InstanceConf) CPUSetQuota(value int) error {
	return c.CPU.SetQuota(value)
}

func (c *InstanceConf) InputDeviceAppend(opts InputDeviceProperties) error {
	if err := opts.Validate(true); err != nil {
		return err
	}

	dev := InputDevice{InputDeviceProperties: opts}

	if err := c.InputDevices.Append(&dev); err != nil {
		if errors.Is(err, pool.ErrAlreadyExists) {
			return &AlreadyConnectedError{"instance_conf", dev.Type}
		}

		return err
	}

	return nil
}

func (c *InstanceConf) InputDeviceRemove(devtype string) error {
	err := c.InputDevices.Remove(devtype)

	if errors.Is(err, pool.ErrNotFound) {
		return &NotConnectedError{"instance_conf", devtype}
	}

	return err
}

func (c *InstanceConf) CdromAppend(opts CdromProperties) error {
	if err := opts.Validate(true); err != nil {
		return err
	}

	cd := Cdrom{
		CdromProperties: opts,
		driver:          CdromDriverTypeValue(opts.Driver),
	}

	if len(cd.Media) > 0 {
		if be, err := NewDiskBackend(cd.Media); err == nil {
			cd.MediaBackend = be
		} else {
			return err
		}
	}

	if err := c.Cdroms.Append(&cd); err != nil {
		if errors.Is(err, pool.ErrAlreadyExists) {
			return &AlreadyConnectedError{"instance_conf", cd.Name}
		}
		return err
	}

	return nil
}

func (c *InstanceConf) CdromInsert(opts CdromProperties, position int) error {
	if err := opts.Validate(true); err != nil {
		return err
	}

	cd := Cdrom{
		CdromProperties: opts,
		driver:          CdromDriverTypeValue(opts.Driver),
	}

	if len(cd.Media) > 0 {
		if be, err := NewDiskBackend(cd.Media); err == nil {
			cd.MediaBackend = be
		} else {
			return err
		}
	}

	if err := c.Cdroms.Insert(&cd, position); err != nil {
		if errors.Is(err, pool.ErrAlreadyExists) {
			return &AlreadyConnectedError{"instance_conf", cd.Name}
		}
		return err
	}

	return nil
}

func (c *InstanceConf) CdromRemove(devname string) error {
	err := c.Cdroms.Remove(devname)

	if errors.Is(err, pool.ErrNotFound) {
		return &NotConnectedError{"instance_conf", devname}
	}

	return err
}

func (c *InstanceConf) CdromChangeMedia(devname, media string) error {
	media = strings.TrimSpace(media)

	if len(media) == 0 {
		return fmt.Errorf("empty cdrom media")
	}

	cd := c.Cdroms.Get(devname)

	if cd == nil {
		return &NotConnectedError{"instance_conf", devname}
	}

	if cd.Media == media {
		return &AlreadyConnectedError{"instance_conf", media}
	}

	if be, err := NewCdromBackend(media); err == nil {
		cd.MediaBackend = be
	} else {
		return err
	}

	cd.Media = media

	return nil
}

func (c *InstanceConf) CdromRemoveMedia(devname string) error {
	cd := c.Cdroms.Get(devname)

	if cd == nil {
		return &NotConnectedError{"instance_conf", devname}
	}

	cd.Media = ""
	cd.MediaBackend = nil

	return nil
}

func (c *InstanceConf) DiskAppend(opts DiskProperties) error {
	if err := opts.Validate(true); err != nil {
		return err
	}

	d := Disk{
		DiskProperties: opts,
		driver:         DiskDriverTypeValue(opts.Driver),
	}

	if be, err := NewDiskBackend(d.Path); err == nil {
		d.Backend = be
	} else {
		return err
	}

	if err := c.Disks.Append(&d); err != nil {
		if errors.Is(err, pool.ErrAlreadyExists) {
			return &AlreadyConnectedError{"instance_conf", d.Path}
		}
		return err
	}

	return nil
}

func (c *InstanceConf) DiskInsert(opts DiskProperties, position int) error {
	if err := opts.Validate(true); err != nil {
		return err
	}

	d := Disk{
		DiskProperties: opts,
		driver:         DiskDriverTypeValue(opts.Driver),
	}

	if be, err := NewDiskBackend(d.Path); err == nil {
		d.Backend = be
	} else {
		return err
	}

	if err := c.Disks.Insert(&d, position); err != nil {
		if errors.Is(err, pool.ErrAlreadyExists) {
			return &AlreadyConnectedError{"instance_conf", d.Path}
		}
		return err
	}

	return nil
}

func (c *InstanceConf) DiskRemove(diskname string) error {
	err := c.Disks.Remove(diskname)

	if errors.Is(err, pool.ErrNotFound) {
		return &NotConnectedError{"instance_conf", diskname}
	}

	return err
}

func (c *InstanceConf) DiskSetReadIops(diskname string, iops int) error {
	if iops < 0 {
		return fmt.Errorf("invalid iops value: cannot be less than 0")
	}

	d := c.Disks.Get(diskname)

	if d == nil {
		return &NotConnectedError{"instance_conf", diskname}
	}

	d.IopsRd = iops

	return nil
}

func (c *InstanceConf) DiskSetWriteIops(diskname string, iops int) error {
	if iops < 0 {
		return fmt.Errorf("invalid iops value: cannot be less than 0")
	}

	d := c.Disks.Get(diskname)

	if d == nil {
		return &NotConnectedError{"instance_conf", diskname}
	}

	d.IopsWr = iops

	return nil
}

func (c InstanceConf) DiskRemoveQemuBitmap(_ string) error {
	return ErrNotImplemented
}

func (c InstanceConf) DiskResizeQemuBlockdev(_ string) error {
	return ErrNotImplemented
}

func (c *InstanceConf) NetIfaceAppend(opts NetIfaceProperties) error {
	if err := opts.Validate(true); err != nil {
		return err
	}

	n := NetIface{
		NetIfaceProperties: opts,
		driver:             NetDriverTypeValue(opts.Driver),
	}

	if err := c.NetIfaces.Append(&n); err != nil {
		if errors.Is(err, pool.ErrAlreadyExists) {
			return &AlreadyConnectedError{"instance_conf", n.Ifname}
		}
		return err
	}

	return nil
}

func (c *InstanceConf) NetIfaceRemove(ifname string) error {
	err := c.NetIfaces.Remove(ifname)

	if errors.Is(err, pool.ErrNotFound) {
		return &NotConnectedError{"instance_conf", ifname}
	}

	return err
}

func (c *InstanceConf) NetIfaceSetQueues(ifname string, queues int) error {
	if queues < 1 {
		return fmt.Errorf("invalid queues value: cannot be less than 1")
	}

	n := c.NetIfaces.Get(ifname)

	if n == nil {
		return &NotConnectedError{"instance_conf", ifname}
	}

	n.Queues = queues

	return nil
}

func (c *InstanceConf) NetIfaceSetUpScript(ifname, scriptPath string) error {
	if _, err := os.Stat(scriptPath); err != nil {
		if os.IsNotExist(err) {
			return err
		}
		return fmt.Errorf("failed to check %s: %w", scriptPath, err)
	}

	n := c.NetIfaces.Get(ifname)

	if n == nil {
		return &NotConnectedError{"instance_conf", ifname}
	}

	n.Ifup = scriptPath

	return nil
}

func (c *InstanceConf) NetIfaceSetDownScript(ifname, scriptPath string) error {
	if _, err := os.Stat(scriptPath); err != nil {
		if os.IsNotExist(err) {
			return err
		}
		return fmt.Errorf("failed to check %s: %w", scriptPath, err)
	}

	n := c.NetIfaces.Get(ifname)

	if n == nil {
		return &NotConnectedError{"instance_conf", ifname}
	}

	n.Ifdown = scriptPath

	return nil
}

func (c *InstanceConf) NetIfaceSetLinkUp(_ string) error {
	return ErrNotImplemented
}

func (c *InstanceConf) NetIfaceSetLinkDown(_ string) error {
	return ErrNotImplemented
}

func (c *InstanceConf) VSockDeviceAppend(opts ChannelVSockProperties) error {
	if c.VSockDevice != nil {
		return &AlreadyConnectedError{"instance_conf", "vsock device"}
	}

	if err := opts.Validate(true); err != nil {
		return err
	}

	vsock := ChannelVSock{
		ChannelVSockProperties: opts,
	}

	c.VSockDevice = &vsock

	return nil
}

func (c *InstanceConf) VSockDeviceRemove() error {
	if c.VSockDevice == nil {
		return &NotConnectedError{"instance_conf", "vsock device"}
	}

	c.VSockDevice = nil

	return nil
}

func (c *InstanceConf) CloudInitSetMedia(media string) error {
	newdrive, err := NewCloudInitDrive(media)
	if err != nil {
		return err
	}

	if _, ok := newdrive.Backend.(*file.Device); ok {
		if filepath.Dir(newdrive.Media) != filepath.Join(CONFDIR, c.name) {
			return fmt.Errorf("must be placed in the machine home directory: %s/", filepath.Join(CONFDIR, c.name))
		}
	}

	if c.CloudInitDrive != nil {
		newdrive.CloudInitDriveProperties.Driver = c.CloudInitDrive.Driver().String()
		newdrive.driver = c.CloudInitDrive.Driver()
	}

	c.CloudInitDrive = newdrive

	return nil
}

func (c *InstanceConf) CloudInitSetDriver(driverName string) error {
	if c.CloudInitDrive == nil {
		return &NotConnectedError{"instance_conf", "cloud-init drive"}
	}

	driver := CloudInitDriverTypeValue(strings.TrimSpace(driverName))

	if driver == DriverType_UNKNOWN {
		return fmt.Errorf("unknown cloud-init driver: %s", driverName)
	}

	c.CloudInitDrive.CloudInitDriveProperties.Driver = driver.String()
	c.CloudInitDrive.driver = driver

	return nil
}

func (c *InstanceConf) CloudInitRemoveConf() error {
	c.CloudInitDrive = nil

	return nil
}

func (c *InstanceConf) KernelSetImage(value string) error {
	return c.Kernel.SetImage(value)
}

func (c *InstanceConf) KernelSetCmdline(value string) error {
	return c.Kernel.SetCmdline(value)
}

func (c *InstanceConf) KernelSetInitrd(value string) error {
	return c.Kernel.SetInitrd(value)
}

func (c *InstanceConf) KernelSetModiso(value string) error {
	return c.Kernel.SetModiso(value)
}

func (c *InstanceConf) KernelRemoveConf() error {
	return c.Kernel.Reset()
}

func (c *InstanceConf) HostDeviceAppend(opts HostDeviceProperties) error {
	dev, err := NewHostDevice(opts.PCIAddr)
	if err != nil {
		return err
	}

	dev.PrimaryGPU = opts.PrimaryGPU
	dev.Multifunction = opts.Multifunction

	if err := c.HostDevices.Append(dev); err != nil {
		if errors.Is(err, pool.ErrAlreadyExists) {
			return &AlreadyConnectedError{"instance_conf", dev.PCIAddr}
		}

		return err
	}

	return nil
}

func (c *InstanceConf) HostDeviceRemove(hexaddr string) error {
	err := c.HostDevices.Remove(hexaddr)

	if errors.Is(err, pool.ErrNotFound) {
		return &NotConnectedError{"instance_conf", hexaddr}
	}

	return err
}

func (c *InstanceConf) HostDeviceSetMultifunctionOption(hexaddr string, enabled bool) error {
	dev := c.HostDevices.Get(hexaddr)

	if dev == nil {
		return &NotConnectedError{"instance_conf", hexaddr}
	}

	dev.Multifunction = enabled

	return nil
}

func (c *InstanceConf) HostDeviceSetPrimaryGPUOption(hexaddr string, enabled bool) error {
	dev := c.HostDevices.Get(hexaddr)

	if dev == nil {
		return &NotConnectedError{"instance_conf", hexaddr}
	}

	dev.PrimaryGPU = enabled

	return nil
}

func (c InstanceConf) VNCSetPassword(s string) error {
	return ErrNotImplemented
}

// IncomingConf represents a virtual machine configuration
// that is used to launch a QEMU incoming instance.
type IncomingConf struct {
	InstanceConf
}

func NewIncomingConf(vmname string) Instance {
	c := IncomingConf{
		InstanceConf{
			InstanceProperties: &InstanceProperties{
				name: vmname,
			},
			confname: "incoming_config",
		},
	}

	return Instance(&c)
}

func GetIncomingConf(vmname string) (Instance, error) {
	vmuser, err := user.Lookup(vmname)
	if err != nil {
		return nil, err
	}

	uid, err := strconv.Atoi(vmuser.Uid)
	if err != nil {
		return nil, err
	}

	c := IncomingConf{
		InstanceConf{
			InstanceProperties: &InstanceProperties{
				name: vmname,
				uid:  uid,
			},
			confname: "incoming_config",
		},
	}

	if b, err := os.ReadFile(c.config()); err == nil {
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	// If a path to persistent flash is set, flashDisk must not be nil.
	if c.Firmware != nil && len(c.Firmware.Flash) > 0 && c.Firmware.flashDisk == nil {
		return nil, &backend.UnknownBackendError{Path: c.Firmware.Flash}
	}

	// Each cdrom device must have a non-nil backend.
	for _, cd := range c.Cdroms.Values() {
		if len(cd.Media) > 0 && cd.MediaBackend == nil {
			return nil, &backend.UnknownBackendError{Path: cd.Media}
		}
	}

	// Each disk device must have a non-nil backend.
	for _, d := range c.Disks.Values() {
		if d.Backend == nil {
			return nil, &backend.UnknownBackendError{Path: d.Path}
		}
	}

	// CloudInit drive must have a non-nil backend.
	if c.CloudInitDrive != nil && c.CloudInitDrive.Backend == nil {
		return nil, &backend.UnknownBackendError{Path: c.CloudInitDrive.Media}
	}

	return &c, nil
}

// StartupConf represents a virtual machine configuration
// that was used to launch a QEMU instance.
type StartupConf struct {
	InstanceConf
}

func GetStartupConf(vmname string) (Instance, error) {
	c := StartupConf{
		InstanceConf{
			InstanceProperties: &InstanceProperties{
				name: vmname,
			},
			confname: "startup_config",
		},
	}

	if b, err := os.ReadFile(c.config()); err == nil {
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	// If a path to persistent flash is set, flashDisk must not be nil.
	if c.Firmware != nil && len(c.Firmware.Flash) > 0 && c.Firmware.flashDisk == nil {
		return nil, &backend.UnknownBackendError{Path: c.Firmware.Flash}
	}

	// Each cdrom device must have a non-nil backend.
	for _, cd := range c.Cdroms.Values() {
		if len(cd.Media) > 0 && cd.MediaBackend == nil {
			return nil, &backend.UnknownBackendError{Path: cd.Media}
		}
	}

	// Each disk device must have a non-nil backend.
	for _, d := range c.Disks.Values() {
		if d.Backend == nil {
			return nil, &backend.UnknownBackendError{Path: d.Path}
		}
	}

	// CloudInit drive must have a non-nil backend.
	if c.CloudInitDrive != nil && c.CloudInitDrive.Backend == nil {
		return nil, &backend.UnknownBackendError{Path: c.CloudInitDrive.Media}
	}

	// Each host-PCI device must have a non-nil backend address.
	for _, dev := range c.HostDevices.Values() {
		if dev.BackendAddr == nil {
			return nil, fmt.Errorf("invalid host-pci addr: %s", dev.PCIAddr)
		}
	}

	return &c, nil
}

func (c StartupConf) config() string {
	return filepath.Join(CHROOTDIR, c.name, "run", c.confname)
}

func (c StartupConf) Save() error {
	return ErrNotImplemented
}
