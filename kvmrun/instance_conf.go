package kvmrun

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// InstanceConf represents a virtual machine configuration
// that is used to prepare a QEMU command line.
type InstanceConf struct {
	*InstanceProperties

	confname string `json:"-"`
}

func NewInstanceConf(vmname string) Instance {
	allowed := regexp.MustCompile(`^[0-9A-Za-z_]{3,16}$`)

	if !allowed.MatchString(vmname) {

	}

	vmc := InstanceConf{
		InstanceProperties: &InstanceProperties{
			name: vmname,
		},
		confname: "config",
	}

	vmc.Mem.Total = 128
	vmc.Mem.Actual = 128
	vmc.CPU.Total = 1
	vmc.CPU.Actual = 1

	return Instance(&vmc)
}

func GetInstanceConf(vmname string) (Instance, error) {
	vmc := InstanceConf{
		InstanceProperties: &InstanceProperties{
			name: vmname,
		},
		confname: "config",
	}

	vmc.Mem.Total = 128
	vmc.Mem.Actual = 128
	vmc.CPU.Total = 1
	vmc.CPU.Actual = 1

	b, err := ioutil.ReadFile(vmc.config())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &NotFoundError{vmname}
		} else {
			return nil, err
		}
	}
	if err := json.Unmarshal(b, &vmc); err != nil {
		return nil, err
	}

	vmc.MachineType = strings.TrimSpace(strings.ToLower(vmc.MachineType))

	for idx, _ := range vmc.Disks {
		b, err := NewDiskBackend(vmc.Disks[idx].Path)
		if err != nil {
			return nil, err
		}
		vmc.Disks[idx].Backend = b
	}

	vmuser, err := user.Lookup(vmname)
	if err != nil {
		return nil, err
	}
	uid, err := strconv.Atoi(vmuser.Uid)
	if err != nil {
		return nil, err
	}
	vmc.uid = uid

	return Instance(&vmc), nil
}

func (c InstanceConf) IsIncoming() bool {
	return c.confname == "incoming_config"
}

func (c InstanceConf) Save() error {
	b, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(c.config(), b, 0644)
}

func (c InstanceConf) SaveStartupConfig() error {
	b, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(CHROOTDIR, c.name, "run/startup_config"), b, 0644)
}

func (c InstanceConf) config() string {
	return filepath.Join(CONFDIR, c.name, c.confname)
}

func (c InstanceConf) Status() (InstanceState, error) {
	return StateInactive, nil
}

func (c InstanceConf) Pid() int {
	return 0
}

func (c *InstanceConf) SetActualCPUs(n int) error {
	if n < 1 {
		return fmt.Errorf("invalid cpu count: cannot be less than 1")
	}
	if n > c.CPU.Total {
		return fmt.Errorf("invalid actual cpu: cannot be large than total cpu (%d)", c.CPU.Total)
	}

	c.CPU.Actual = n

	return nil
}

func (c *InstanceConf) SetTotalCPUs(n int) error {
	if n < 1 {
		return fmt.Errorf("invalid cpu count: cannot be less than 1")
	}
	if n < c.CPU.Actual {
		return fmt.Errorf("invalid total cpu: cannot be less than actual cpu")
	}

	c.CPU.Total = n

	return nil
}

func (c *InstanceConf) SetCPUSockets(n int) error {
	if n < 0 {
		return fmt.Errorf("invalid number of processor sockets: cannot be less than 0")
	}

	if c.CPU.Total%n != 0 {
		return fmt.Errorf("invalid number of processor sockets: total cpu count must be multiple of %d", n)
	}

	c.CPU.Sockets = n

	return nil
}

func (c *InstanceConf) SetCPUModel(model string) error {
	c.CPU.Model = model
	return nil
}

func (c *InstanceConf) SetCPUQuota(quota int) error {
	c.CPU.Quota = quota
	return nil
}

func (c *InstanceConf) SetMachineType(t string) error {
	c.MachineType = t
	return nil
}

func (c *InstanceConf) SetActualMem(s int) error {
	if s < 1 {
		return fmt.Errorf("invalid memory size: cannot be less than 1")
	}
	if s > c.Mem.Total {
		return fmt.Errorf("invalid actual memory: cannot be large than total memory (%d)", c.Mem.Total)
	}

	c.Mem.Actual = s

	return nil
}

func (c *InstanceConf) SetTotalMem(s int) error {
	if s < 1 {
		return fmt.Errorf("invalid memory size: cannot be less than 1")
	}
	if s < c.Mem.Actual {
		return fmt.Errorf("invalid total memory: cannot be less than actual memory")
	}

	c.Mem.Total = s

	return nil
}

func (c *InstanceConf) AppendInputDevice(d InputDevice) error {
	if c.Inputs.Exists(d.Type) {
		return &AlreadyConnectedError{"instance_conf", d.Type}
	}

	c.Inputs.Append(&d)

	return nil
}

func (c *InstanceConf) RemoveInputDevice(t string) error {
	if c.Inputs.Exists(t) {
		return c.Inputs.Remove(t)
	}

	return &NotConnectedError{"instance_conf", t}
}

func (c *InstanceConf) AppendCdrom(d Cdrom) error {
	if c.Cdroms.Exists(d.Name) {
		return &AlreadyConnectedError{"instance_conf", d.Name}
	}

	if !CdromDrivers.Exists(d.Driver) {
		return fmt.Errorf("unknown disk driver: %s", d.Driver)
	}

	c.Cdroms.Append(&d)

	return nil
}

func (c *InstanceConf) InsertCdrom(d Cdrom, idx int) error {
	if c.Cdroms.Exists(d.Name) {
		return &AlreadyConnectedError{"instance_conf", d.Name}
	}

	if !CdromDrivers.Exists(d.Driver) {
		return fmt.Errorf("unknown disk driver: %s", d.Driver)
	}

	if idx > len(c.Cdroms) {
		idx = len(c.Cdroms)
	}

	if err := c.Cdroms.Insert(&d, idx); err != nil {
		return err
	}

	return nil
}

func (c *InstanceConf) RemoveCdrom(name string) error {
	if c.Cdroms.Exists(name) {
		return c.Cdroms.Remove(name)
	}

	return &NotConnectedError{"instance_conf", name}
}

func (c *InstanceConf) ChangeCdromMedia(name, media string) error {
	d := c.Cdroms.Get(name)
	if d == nil {
		return &NotConnectedError{"instance_conf", name}
	}

	if b, err := NewCdromBackend(media); err == nil {
		d.Backend = b
	} else {
		return err
	}

	d.Media = media

	return nil
}

func (c InstanceConf) ResizeQemuBlockdev(_ string) error {
	return ErrNotImplemented
}

func (c *InstanceConf) AppendDisk(d Disk) error {
	if !DiskDrivers.Exists(d.Driver) {
		return fmt.Errorf("unknown disk driver: %s", d.Driver)
	}

	if c.Disks.Exists(d.Path) {
		return &AlreadyConnectedError{"instance_conf", d.Path}
	}

	c.Disks.Append(&d)

	return nil
}

func (c *InstanceConf) InsertDisk(d Disk, idx int) error {
	if !DiskDrivers.Exists(d.Driver) {
		return fmt.Errorf("unknown disk driver: %s", d.Driver)
	}

	if idx > len(c.Disks) {
		idx = len(c.Disks)
	}

	if c.Disks.Exists(d.Path) {
		return &AlreadyConnectedError{"instance_conf", d.Path}
	}

	if err := c.Disks.Insert(&d, idx); err != nil {
		return err
	}

	return nil
}

func (c *InstanceConf) RemoveDisk(dpath string) error {
	if !c.Disks.Exists(dpath) {
		return &NotConnectedError{"instance_conf", dpath}
	}

	return c.Disks.Remove(dpath)
}

func (c *InstanceConf) AppendProxy(proxy Proxy) error {
	if c.Proxy.Exists(proxy.Path) {
		return &AlreadyConnectedError{"instance_conf", proxy.Path}
	}

	c.Proxy.Append(&proxy)

	return nil
}

func (c *InstanceConf) RemoveProxy(fullpath string) error {
	if !c.Proxy.Exists(fullpath) {
		return &NotConnectedError{"instance_conf", fullpath}
	}

	return c.Proxy.Remove(fullpath)
}

func (c *InstanceConf) SetDiskReadIops(dpath string, iops int) error {
	if iops < 0 {
		return fmt.Errorf("invalid iops value: cannot be less than 0")
	}

	d := c.Disks.Get(dpath)
	if d == nil {
		return &NotConnectedError{"instance_conf", dpath}
	}

	d.IopsRd = iops

	return nil
}

func (c *InstanceConf) SetDiskWriteIops(dpath string, iops int) error {
	if iops < 0 {
		return fmt.Errorf("invalid iops value: cannot be less than 0")
	}

	d := c.Disks.Get(dpath)
	if d == nil {
		return &NotConnectedError{"instance_conf", dpath}
	}

	d.IopsWr = iops

	return nil
}

func (c InstanceConf) RemoveDiskBitmap(dpath string) error {
	return ErrNotImplemented
}

func (c *InstanceConf) AppendNetIface(iface NetIface) error {
	if len(iface.Ifname) == 0 {
		return fmt.Errorf("undefined network interface name")
	}

	if !NetDrivers.Exists(iface.Driver) {
		return fmt.Errorf("unknown network interface driver: %s", iface.Driver)
	}

	if c.NetIfaces.Exists(iface.Ifname) {
		return &AlreadyConnectedError{"instance_conf", iface.Ifname}
	}

	if _, err := net.ParseMAC(iface.HwAddr); err != nil {
		return err
	}

	c.NetIfaces.Append(&iface)

	return nil
}

func (c *InstanceConf) RemoveNetIface(ifname string) error {
	if !c.NetIfaces.Exists(ifname) {
		return &NotConnectedError{"instance_conf", ifname}
	}

	return c.NetIfaces.Remove(ifname)
}

func (c *InstanceConf) SetNetIfaceUpScript(ifname, scriptPath string) error {
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("file not found: %s", scriptPath)
	}

	n := c.NetIfaces.Get(ifname)
	if n == nil {
		return &NotConnectedError{"instance_conf", ifname}
	}

	n.Ifup = scriptPath

	return nil
}

func (c *InstanceConf) SetNetIfaceDownScript(ifname, scriptPath string) error {
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("file not found: %s", scriptPath)
	}

	n := c.NetIfaces.Get(ifname)
	if n == nil {
		return &NotConnectedError{"instance_conf", ifname}
	}

	n.Ifdown = scriptPath

	return nil
}

func (c *InstanceConf) SetNetIfaceLinkUp(ifname string) error {
	return ErrNotImplemented
}

func (c *InstanceConf) SetNetIfaceLinkDown(ifname string) error {
	return ErrNotImplemented
}

func (c *InstanceConf) AppendVSockDevice(cid uint32) error {
	if c.VSockDevice != nil {
		return &AlreadyConnectedError{"instance_conf", "vsock device"}
	}

	vsock := new(VirtioVSock)

	switch {
	case cid == 0:
		vsock.Auto = true
	case cid >= 3:
		vsock.ContextID = cid
	default:
		return ErrIncorrectContextID
	}

	c.VSockDevice = vsock

	return nil
}

func (c *InstanceConf) RemoveVSockDevice() error {
	if c.VSockDevice == nil {
		return &NotConnectedError{"instance_conf", "vsock device"}
	}

	c.VSockDevice = nil

	return nil
}

func (c *InstanceConf) SetCloudInitDrive(s string) error {
	c.CIDrive.Path = s

	return nil
}

func (c *InstanceConf) RemoveKernelConf() error {
	c.Kernel.Image = ""
	c.Kernel.Cmdline = ""
	c.Kernel.Initrd = ""
	c.Kernel.Modiso = ""

	return nil
}

func (c *InstanceConf) SetKernelImage(s string) error {
	c.Kernel.Image = s

	return nil
}

func (c *InstanceConf) SetKernelCmdline(s string) error {
	c.Kernel.Cmdline = s
	return nil
}

func (c *InstanceConf) SetKernelInitrd(s string) error {
	c.Kernel.Initrd = s
	return nil
}

func (c *InstanceConf) SetKernelModiso(s string) error {
	c.Kernel.Modiso = s
	return nil
}

func (c InstanceConf) SetVNCPassword(s string) error {
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

	b, err := ioutil.ReadFile(c.config())
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}

	for idx, _ := range c.Disks {
		b, err := NewDiskBackend(c.Disks[idx].Path)
		if err != nil {
			return nil, err
		}
		c.Disks[idx].Backend = b
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

	b, err := ioutil.ReadFile(c.config())
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}

	for idx, _ := range c.Disks {
		b, err := NewDiskBackend(c.Disks[idx].Path)
		if err != nil {
			return nil, err
		}
		c.Disks[idx].Backend = b
	}

	return &c, nil
}

func (c StartupConf) config() string {
	return filepath.Join(CHROOTDIR, c.name, "run", c.confname)
}

func (c StartupConf) Save() error {
	return ErrNotImplemented
}
