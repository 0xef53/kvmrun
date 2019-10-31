package kvmrun

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

// InstanceConf represents a virtual machine configuration
// that is used to prepare a QEMU command line.
type InstanceConf struct {
	name      string    `json:"-"`
	Mem       Memory    `json:"memory"`
	CPU       CPU       `json:"cpu"`
	Disks     Disks     `json:"storage"`
	NetIfaces NetIfaces `json:"network"`
	Channels  Channels  `json:"channels"`
	Kernel    ExtKernel `json:"kernel"`
	Machine   string    `json:"machine,omitempty"`
	uid       int       `json:"-"`
	confname  string    `json:"-"`
}

func NewInstanceConf(vmname string) Instance {
	vmc := InstanceConf{
		name:     vmname,
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
		name:     vmname,
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

func (c InstanceConf) Clone() Instance {
	x := InstanceConf{
		name:     c.name,
		Mem:      c.Mem,
		CPU:      c.CPU,
		Kernel:   c.Kernel,
		Machine:  c.Machine,
		uid:      c.uid,
		confname: c.confname,
	}

	x.Disks = c.Disks.Clone()
	x.NetIfaces = c.NetIfaces.Clone()
	//x.Channels = c.Channels.Clone()

	return Instance(&x)
}

func (c InstanceConf) IsIncoming() bool {
	return c.confname == "incoming_config"
}

func (c InstanceConf) Save() error {
	b, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(c.config(), b, 0644); err != nil {
		return err
	}

	return nil
}

func (c InstanceConf) SaveStartupConfig() error {
	b, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}

	fname := filepath.Join(CHROOTDIR, c.name, "run/startup_config")
	if err := ioutil.WriteFile(fname, b, 0644); err != nil {
		return err
	}

	return nil
}

func (c InstanceConf) config() string {
	return filepath.Join(VMCONFDIR, c.name, c.confname)
}

func (c InstanceConf) Name() string {
	return c.name
}

func (c InstanceConf) Uid() int {
	return c.uid
}

func (c InstanceConf) Status() (string, error) {
	switch b, err := ioutil.ReadFile(filepath.Join(VMCONFDIR, c.name, "supervise/migration_stat")); {
	case err == nil:
		st := struct {
			Status string
		}{}
		if err := json.Unmarshal(b, &st); err != nil {
			return "", err
		}
		switch st.Status {
		case "migrated", "completed":
			return "migrated", nil
		}
	case os.IsNotExist(err):
	case err != nil:
		return "", err
	}

	return "configured", nil
}

func (c InstanceConf) Pid() int {
	return -1
}

func (c InstanceConf) GetActualCPUs() int {
	return c.CPU.Actual
}

func (c *InstanceConf) SetActualCPUs(n int) error {
	if n < 1 {
		return fmt.Errorf("Invalid cpu count: cannot be less than 1")
	}
	if n > c.CPU.Total {
		return fmt.Errorf("Invalid actual cpu: cannot be large than total cpu (%d)", c.CPU.Total)
	}

	c.CPU.Actual = n

	return nil
}

func (c InstanceConf) GetTotalCPUs() int {
	return c.CPU.Total
}

func (c *InstanceConf) SetTotalCPUs(n int) error {
	if n < 1 {
		return fmt.Errorf("Invalid cpu count: cannot be less than 1")
	}
	if n < c.CPU.Actual {
		return fmt.Errorf("Invalid total cpu: cannot be less than actual cpu")
	}

	c.CPU.Total = n

	return nil
}

func (c InstanceConf) GetCPUModel() string {
	return c.CPU.Model
}

func (c *InstanceConf) SetCPUModel(model string) error {
	c.CPU.Model = model
	return nil
}

func (c InstanceConf) GetCPUQuota() int {
	return c.CPU.Quota
}

func (c *InstanceConf) SetCPUQuota(quota int) error {
	c.CPU.Quota = quota
	return nil
}

func (c InstanceConf) GetMachineType() string {
	return c.Machine
}

func (c *InstanceConf) SetMachineType(t string) error {
	c.Machine = t
	return nil
}

func (c InstanceConf) GetActualMem() int {
	return c.Mem.Actual
}

func (c *InstanceConf) SetActualMem(s int) error {
	if s < 1 {
		return fmt.Errorf("Invalid memory size: cannot be less than 1")
	}
	if s > c.Mem.Total {
		return fmt.Errorf("Invalid actual memory: cannot be large than total memory (%d)", c.Mem.Total)
	}

	c.Mem.Actual = s

	return nil
}

func (c InstanceConf) GetTotalMem() int {
	return c.Mem.Total
}

func (c *InstanceConf) SetTotalMem(s int) error {
	if s < 1 {
		return fmt.Errorf("Invalid memory size: cannot be less than 1")
	}
	if s < c.Mem.Actual {
		return fmt.Errorf("Invalid total memory: cannot be less than actual memory")
	}

	c.Mem.Total = s

	return nil
}

func (c InstanceConf) GetDisks() Disks {
	// TODO: Clone()
	dd := make(Disks, len(c.Disks))
	copy(dd, c.Disks)
	return dd
}

func (c InstanceConf) ResizeDisk(dpath string) error {
	return ErrNotImplemented
}

func (c *InstanceConf) AppendDisk(d Disk) error {
	if !DiskDrivers.Exists(d.Driver) {
		return fmt.Errorf("Unknown disk driver: %s", d.Driver)
	}

	if c.Disks.Exists(d.Path) {
		return &AlreadyConnectedError{"instance_conf", d.Path}
	}

	c.Disks.Append(&d)

	return nil
}

func (c *InstanceConf) InsertDisk(d Disk, idx int) error {
	if !DiskDrivers.Exists(d.Driver) {
		return fmt.Errorf("Unknown disk driver: %s", d.Driver)
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

func (c *InstanceConf) SetDiskReadIops(dpath string, iops int) error {
	if iops < 0 {
		return fmt.Errorf("Invalid iops value: cannot be less than 0")
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
		return fmt.Errorf("Invalid iops value: cannot be less than 0")
	}

	d := c.Disks.Get(dpath)
	if d == nil {
		return &NotConnectedError{"instance_conf", dpath}
	}

	d.IopsWr = iops

	return nil
}

func (c InstanceConf) GetNetIfaces() NetIfaces {
	nn := make(NetIfaces, len(c.NetIfaces))
	copy(nn, c.NetIfaces)
	return nn
}

func (c *InstanceConf) AppendNetIface(iface NetIface) error {
	if !NetDrivers.Exists(iface.Driver) {
		return fmt.Errorf("Unknown network interface driver: %s", iface.Driver)
	}

	if c.NetIfaces.Exists(iface.Ifname) {
		return &AlreadyConnectedError{"instance_conf", iface.Ifname}
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
		return fmt.Errorf("File not found: %s", scriptPath)
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
		return fmt.Errorf("File not found: %s", scriptPath)
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

func (c InstanceConf) GetChannels() Channels {
	cc := make(Channels, len(c.Channels))
	copy(cc, c.Channels)
	return cc
}

func (c *InstanceConf) AppendChannel(ch VirtioChannel) error {
	if c.Channels.Exists(ch.ID) {
		return &AlreadyConnectedError{"instance_conf", ch.ID}
	}
	if c.Channels.NameExists(ch.Name) {
		return &AlreadyConnectedError{"instance_conf", ch.Name}
	}

	c.Channels.Append(&ch)

	return nil
}

func (c *InstanceConf) RemoveChannel(id string) error {
	if !c.Channels.Exists(id) {
		return &NotConnectedError{"instance_conf", id}
	}
	return c.Channels.Remove(id)
}

func (c InstanceConf) GetKernelImage() string {
	return c.Kernel.Image
}

func (c InstanceConf) GetKernelCmdline() string {
	return c.Kernel.Cmdline
}

func (c InstanceConf) GetKernelInitrd() string {
	return c.Kernel.Initrd
}

func (c InstanceConf) GetKernelModiso() string {
	return c.Kernel.Modiso
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
			name:     vmname,
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
			name:     vmname,
			uid:      uid,
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
			name:     vmname,
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
