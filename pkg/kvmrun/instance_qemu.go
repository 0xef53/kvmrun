package kvmrun

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	qmp "github.com/0xef53/go-qmp"

	cg "github.com/0xef53/kvmrun/pkg/cgroup"
	"github.com/0xef53/kvmrun/pkg/ps"
	qt "github.com/0xef53/kvmrun/pkg/qemu/types"
)

// InstanceQemu represents a configuration of a running QEMU instance.
type InstanceQemu struct {
	name       string                  `json:"-"`
	Mem        Memory                  `json:"memory"`
	CPU        CPU                     `json:"cpu"`
	Disks      Disks                   `json:"storage"`
	NetIfaces  NetIfaces               `json:"network"`
	Channels   Channels                `json:"channels"`
	Kernel     ExtKernel               `json:"kernel"`
	Machine    string                  `json:"machine,omitempty"`
	scsiBuses  map[string]*SCSIBusInfo `json:"-"`
	uid        int                     `json:"-"`
	pid        int                     `json:"-"`
	cmdline    []string                `json:"-"`
	qemuVer    QemuVersion             `json:"-"`
	mon        *qmp.Monitor            `json:"-"`
	startupCfg Instance                `json:"-"`
}

func GetInstanceQemu(vmname string, mon *qmp.Monitor) (Instance, error) {
	if mon == nil {
		return nil, &NotRunningError{vmname}
	}

	vmuser, err := user.Lookup(vmname)
	if err != nil {
		return nil, err
	}
	uid, err := strconv.Atoi(vmuser.Uid)
	if err != nil {
		return nil, err
	}

	r := InstanceQemu{
		name: vmname,
		uid:  uid,
		mon:  mon,
	}

	switch err := r.initVersion(); {
	case err == nil:
	case qmp.IsSocketNotAvailable(err):
		return nil, &NotRunningError{}
	default:
		return nil, err
	}

	if cfg, err := GetStartupConf(vmname); err == nil {
		r.startupCfg = cfg
	} else {
		return nil, fmt.Errorf("Unable to load startup config: %s", err)
	}

	var gr errgroup.Group

	gr.Go(func() error { return r.initProcess() })
	gr.Go(func() error { return r.initStorage() })
	gr.Go(func() error { return r.initNetwork() })
	gr.Go(func() error { return r.initChannels() })
	gr.Go(func() error { return r.initMemory() })
	gr.Go(func() error { return r.initCPU() })
	gr.Go(func() error { return r.initKern() })
	gr.Go(func() error { return r.initMachineType() })

	switch err := gr.Wait(); {
	case err == nil:
	case qmp.IsSocketNotAvailable(err):
		return nil, &NotRunningError{vmname}
	default:
		return nil, err
	}

	return Instance(&r), nil
}

func (r InstanceQemu) Clone() Instance {
	x := InstanceQemu{
		name:       r.name,
		Mem:        r.Mem,
		CPU:        r.CPU,
		Kernel:     r.Kernel,
		Machine:    r.Machine,
		uid:        r.uid,
		pid:        r.pid,
		cmdline:    r.cmdline,
		mon:        r.mon,
		startupCfg: r.startupCfg,
	}

	x.Disks = r.Disks.Clone()
	x.NetIfaces = r.NetIfaces.Clone()
	//x.Channels = r.Channels.Clone()

	return Instance(&x)
}

func (r InstanceQemu) IsIncoming() bool {
	return false
}

func (r InstanceQemu) Save() error {
	return ErrNotImplemented
}

func (r InstanceQemu) Name() string {
	return r.name
}

func (r InstanceQemu) Uid() int {
	return r.uid
}

func (r InstanceQemu) Status() (string, error) {
	// TODO: use inmigrate status when disks are copying

	var status string

	var st qt.StatusInfo
	if err := r.mon.Run(qmp.Command{"query-status", nil}, &st); err != nil {
		return "", err
	}
	switch st.Status {
	case "inmigrate", "postmigrate", "finish-migrate":
		status = "incoming"
	default:
		status = st.Status
	}

	// Checking the current migration status
	migrSt := struct {
		Status    string `json:"status"`
		TotalTime uint64 `json:"total-time"`
	}{}
	if err := r.mon.Run(qmp.Command{"query-migrate", nil}, &migrSt); err != nil {
		return "", err
	}
	switch migrSt.Status {
	case "setup", "cancelling", "active", "postcopy-active", "pre-switchover", "device":
		status = "inmigrate"
	case "completed":
		if migrSt.TotalTime != 0 {
			status = "migrated"
		}
	}

	return status, nil
}

func (r *InstanceQemu) initVersion() error {
	ver := struct {
		Qemu struct {
			Major int `json:"major"`
			Minor int `json:"minor"`
			Micro int `json:"micro"`
		} `json:"qemu"`
	}{}

	if err := r.mon.Run(qmp.Command{"query-version", nil}, &ver); err != nil {
		return err
	}

	// E.g.: 21101 is 2.11.1
	r.qemuVer = QemuVersion(ver.Qemu.Major*10000 + ver.Qemu.Minor*100 + ver.Qemu.Micro)

	return nil
}

func (r *InstanceQemu) initProcess() error {
	var firstThreadID int

	switch {
	case r.qemuVer < 21200:
		cpus := make([]qt.CPUInfo, 0, 8)
		if err := r.mon.Run(qmp.Command{"query-cpus", nil}, &cpus); err != nil {
			return err
		}
		firstThreadID = cpus[0].ThreadID
	default:
		cpus := make([]qt.CPUInfoFast, 0, 8)
		if err := r.mon.Run(qmp.Command{"query-cpus-fast", nil}, &cpus); err != nil {
			return err
		}
		firstThreadID = cpus[0].ThreadID
	}

	f, err := os.Open(filepath.Join("/proc", fmt.Sprintf("%d", firstThreadID), "status"))
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if b := scanner.Bytes(); bytes.HasPrefix(b, []byte("Tgid:")) {
			bf := bytes.Fields(b)
			if len(bf) < 2 {
				continue
			}
			pid, err := strconv.Atoi(string(bf[1]))
			if err != nil {
				return err
			}
			r.pid = pid
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	cmdline, err := ps.GetCmdline(r.pid)
	if err != nil {
		return err
	}
	r.cmdline = cmdline

	return nil
}

func (r InstanceQemu) Pid() int {
	return r.pid
}

func (r *InstanceQemu) initCPU() error {
	// Actual vCPU count
	switch {
	case r.qemuVer < 21200:
		attachedCPUs := make([]qt.CPUInfo, 0, 8)
		if err := r.mon.Run(qmp.Command{"query-cpus", nil}, &attachedCPUs); err != nil {
			return err
		}
		r.CPU.Actual = len(attachedCPUs)
	default:
		attachedCPUs := make([]qt.CPUInfoFast, 0, 8)
		if err := r.mon.Run(qmp.Command{"query-cpus-fast", nil}, &attachedCPUs); err != nil {
			return err
		}
		r.CPU.Actual = len(attachedCPUs)
	}

	// Total vCPU count
	r.CPU.Total = r.startupCfg.GetTotalCPUs()

	// CPU model
	r.CPU.Model = r.startupCfg.GetCPUModel()

	// Cgroups CPU quota
	if g, err := cg.LookupCgroupByPid(r.pid, "cpu"); err == nil {
		if strings.HasSuffix(g.GetPath(), filepath.Join("/kvmrun", r.name)) {
			c := cg.Config{}
			if err := g.Get(&c); err != nil {
				return err
			}
			if c.CpuQuota == 0 || c.CpuQuota == -1 {
				r.CPU.Quota = 0
			} else {
				r.CPU.Quota = int(c.CpuQuota * 100 / c.CpuPeriod)
			}
		}
	}

	return nil
}

func (r InstanceQemu) GetActualCPUs() int {
	return r.CPU.Actual
}

func (r *InstanceQemu) SetActualCPUs(n int) error {
	if n < 1 {
		return fmt.Errorf("Invalid cpu count: cannot be less than 1")
	}
	if n > r.CPU.Total {
		return fmt.Errorf("Invalid actual cpu: cannot be large than total cpu (%d)", r.CPU.Total)
	}

	switch {
	case n < r.CPU.Actual:
		// Decrease
		if r.qemuVer < 20700 {
			return fmt.Errorf("Hot-unplug operation is not supported in QEMU %s", r.qemuVer)
		}

		availableCPUs := make([]qt.HotpluggableCPU, 0, 8)
		if err := r.mon.Run(qmp.Command{"query-hotpluggable-cpus", nil}, &availableCPUs); err != nil {
			return err
		}

		for _, cpu := range availableCPUs {
			if cpu.Props.SocketID >= n && cpu.Props.SocketID < r.CPU.Total && len(cpu.QomPath) > 0 {
				if err := r.mon.Run(qmp.Command{"device_del", qt.StrID{cpu.QomPath}}, nil); err != nil {
					return err
				}

			}
		}
	case n > r.CPU.Actual:
		// Increase
		for i := r.CPU.Actual; i < n; i++ {
			if err := r.mon.Run(qmp.Command{"cpu-add", qt.IntID{i}}, nil); err != nil {
				return err
			}
			r.CPU.Actual++
		}
	}

	return nil
}

func (r InstanceQemu) GetTotalCPUs() int {
	return r.CPU.Total
}

func (r *InstanceQemu) SetTotalCPUs(n int) error {
	return ErrNotImplemented
}

func (r InstanceQemu) GetCPUModel() string {
	return r.CPU.Model
}

func (r *InstanceQemu) SetCPUModel(model string) error {
	return ErrNotImplemented
}

func (r InstanceQemu) GetCPUQuota() int {
	return r.CPU.Quota
}

func (r *InstanceQemu) SetCPUQuota(quota int) error {
	cpuGroup, err := cg.NewCpuGroup(filepath.Join("kvmrun", r.name), r.pid)
	if err != nil {
		return err
	}

	c := cg.Config{}
	if err := cpuGroup.Get(&c); err != nil {
		return err
	}

	// If CPU quota is disabled in Kernel
	if c.CpuPeriod == 0 {
		return cg.ErrCfsNotEnabled
	}

	if quota == 0 {
		c.CpuQuota = -1
	} else {
		c.CpuQuota = (c.CpuPeriod * int64(quota)) / 100
	}
	if err := cpuGroup.Set(&c); err != nil {
		return err
	}

	return nil
}

func (r *InstanceQemu) initMachineType() error {
	var mtype string

	if err := r.mon.Run(qmp.Command{"qom-get", &qt.QomQuery{"/machine", "type"}}, &mtype); err != nil {
		return err
	}

	r.Machine = strings.TrimSuffix(mtype, "-machine")

	return nil
}

func (r InstanceQemu) GetMachineType() string {
	return r.Machine
}

func (r InstanceQemu) SetMachineType(t string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) initMemory() error {
	balloonInfo := struct {
		Actual uint64 `json:"actual"`
	}{}

	if err := r.mon.Run(qmp.Command{"query-balloon", nil}, &balloonInfo); err != nil {
		return err
	}

	r.Mem.Actual = int(balloonInfo.Actual >> 20)
	r.Mem.Total = r.startupCfg.GetTotalMem()

	return nil
}

func (r InstanceQemu) GetActualMem() int {
	return r.Mem.Actual
}

func (r *InstanceQemu) SetActualMem(s int) error {
	if s < 1 {
		return fmt.Errorf("Invalid memory size: cannot be less than 1")
	}
	if s > r.Mem.Total {
		return fmt.Errorf("invalid actual memory: cannot be large than total memory (%d)", r.Mem.Total)
	}

	if err := r.mon.Run(qmp.Command{"balloon", qt.IntValue{s << 20}}, nil); err != nil {
		return err
	}

	r.Mem.Actual = s

	return nil
}

func (r InstanceQemu) GetTotalMem() int {
	return r.Mem.Total
}

func (r *InstanceQemu) SetTotalMem(s int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) initStorage() error {
	pciDevs := make([]qt.PCIInfo, 0, 1)
	if err := r.mon.Run(qmp.Command{"query-pci", nil}, &pciDevs); err != nil {
		return err
	}

	r.scsiBuses = make(map[string]*SCSIBusInfo)
	for _, dev := range pciDevs[0].Devices {
		// desc:      SCSI controller
		// class:     256
		// id.device: 4100
		if !(dev.ClassInfo.Class == 256 && dev.ID.Device == 4100) {
			continue
		}
		r.scsiBuses[dev.QdevID] = &SCSIBusInfo{"virtio-scsi-pci", fmt.Sprintf("0x%x", dev.Slot)}
	}

	blkDevs := make([]qt.BlockInfo, 0, 8)
	if err := r.mon.Run(qmp.Command{"query-block", nil}, &blkDevs); err != nil {
		return err
	}

	pool := make(Disks, 0, len(blkDevs))
	for _, dev := range blkDevs {
		if dev.Device == "modiso" {
			continue
		}
		if dev.Inserted.File == "" {
			continue
		}

		var devicePath string

		if strings.HasPrefix(dev.Inserted.File, "json:") {
			b := qt.InsertedFileOptions{}
			if err := json.Unmarshal([]byte(dev.Inserted.File[5:]), &b); err != nil {
				return err
			}
			switch b.File.Driver {
			case "iscsi":
				devicePath = fmt.Sprintf(
					"iscsi://%s%%%s@%s/%s/%s",
					b.File.User,
					b.File.Password,
					b.File.Portal,
					b.File.Target,
					b.File.Lun,
				)
			default:
				return fmt.Errorf("Unknown backing device driver: %s", b.File.Driver)
			}
		} else {
			devicePath = dev.Inserted.File
		}

		if dev.Inserted.BackingFileDepth > 0 {
			devicePath = dev.Inserted.BackingFile
		}

		disk, err := NewDisk(devicePath)
		if err != nil {
			return err
		}

		disk.IopsRd = dev.Inserted.IopsRd
		disk.IopsWr = dev.Inserted.IopsWr

		if dev.Inserted.BackingFileDepth > 0 {
			disk.QemuVirtualSize = dev.Inserted.Image.BackingImage.VirtualSize
		} else {
			disk.QemuVirtualSize = dev.Inserted.Image.VirtualSize
		}

		if err := r.mon.Run(qmp.Command{"qom-get", &qt.QomQuery{disk.QdevID(), "type"}}, &disk.Driver); err != nil {
			return err
		}
		if !DiskDrivers.Exists(disk.Driver) {
			continue
		}

		switch disk.Driver {
		case "virtio-blk-pci", "ide-hd":
			// An addr/slot on the PCI bus
			var pciAddr string
			if err := r.mon.Run(qmp.Command{"qom-get", &qt.QomQuery{disk.QdevID(), "legacy-addr"}}, &pciAddr); err == nil {
				disk.Addr = fmt.Sprintf("0x%s", strings.Split(pciAddr, ".")[0])
			}
		case "scsi-hd":
			// SCSI bus name/addr and lun of disk
			var parentBus string
			if err := r.mon.Run(qmp.Command{"qom-get", &qt.QomQuery{disk.QdevID(), "parent_bus"}}, &parentBus); err != nil {
				return err
			}
			// in:  /machine/peripheral/scsi0/virtio-backend/scsi0.0
			// out: scsi0
			parentBusName := strings.Split(filepath.Base(parentBus), ".")[0]

			var lun int
			if err := r.mon.Run(qmp.Command{"qom-get", &qt.QomQuery{disk.QdevID(), "lun"}}, &lun); err != nil {
				return err
			}

			disk.Addr = fmt.Sprintf("%s:%s/%d", parentBusName, r.scsiBuses[parentBusName].Addr, lun)
		}
		for _, m := range dev.DirtyBitmaps {
			if m.Name == "backup" {
				disk.HasBitmap = true
			}
		}

		pool = append(pool, *disk)
	}

	r.Disks = pool

	return nil
}

func (r InstanceQemu) GetDisks() Disks {
	// TODO: Clone ?
	dd := make(Disks, len(r.Disks))
	copy(dd, r.Disks)
	return dd
}

func (r InstanceQemu) ResizeDisk(dpath string) error {
	d := r.Disks.Get(dpath)
	if d == nil {
		return &NotConnectedError{"instance_qemu", dpath}
	}

	opts := qt.BlockResizeQuery{
		Device: d.BaseName(),
		Size:   1,
	}

	return r.mon.Run(qmp.Command{"block_resize", &opts}, nil)
}

func (r *InstanceQemu) AppendDisk(d Disk) error {
	if r.Disks.Exists(d.Path) {
		return &AlreadyConnectedError{"instance_qemu", d.Path}
	}

	if !DiskDrivers.HotPluggable(d.Driver) {
		return fmt.Errorf("Unknown hotpuggable disk driver: %s", d.Driver)
	}

	if d.IsLocal() {
		devPath := filepath.Join(CHROOTDIR, r.name, d.Path)
		os.MkdirAll(filepath.Dir(devPath), 0755)
		stat := syscall.Stat_t{}
		if err := syscall.Stat(d.Path, &stat); err != nil {
			return err
		}
		if err := syscall.Mknod(devPath, syscall.S_IFBLK|uint32(os.FileMode(01640)), int(stat.Rdev)); err != nil {
			return err
		}
		if err := os.Chown(devPath, r.uid, 0); err != nil {
			return err
		}
	}

	devOpts := qt.DeviceOptions{
		Driver: d.Driver,
		Id:     d.QdevID(),
		Drive:  d.BaseName(),
	}

	switch d.Driver {
	case "scsi-hd":
		busName, _, _ := ParseSCSIAddr(d.Addr)
		if _, ok := r.scsiBuses[busName]; !ok {
			busOpts := qt.DeviceOptions{
				Driver: "virtio-scsi-pci",
				Id:     busName,
			}
			if err := r.mon.Run(qmp.Command{"device_add", &busOpts}, nil); err != nil {
				return fmt.Errorf("device_add failed: %s", err)
			}
		}
		devOpts.Bus = fmt.Sprintf("%s.0", busName)
		devOpts.SCSI_ID = 1
	}

	// That's a fucking shame that we need to do it exactly that way
	cmd := fmt.Sprintf(
		"drive_add auto \"file=%s,id=%s,format=raw,if=none,aio=native,cache=none,detect-zeroes=on,iops_rd=%d,iops_wr=%d\"",
		d.Path,
		d.BaseName(),
		d.IopsRd,
		d.IopsWr,
	)
	if _, err := r.mon.RunHuman(cmd); err != nil {
		return fmt.Errorf("drive_add failed: %s", err)
	}

	if err := r.mon.Run(qmp.Command{"device_add", &devOpts}, nil); err != nil {
		return fmt.Errorf("device_add failed: %s", err)
	}

	r.Disks.Append(&d)

	return nil
}

func (r *InstanceQemu) InsertDisk(d Disk, index int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) RemoveDisk(dpath string) error {
	d := r.Disks.Get(dpath)
	if d == nil {
		return &NotConnectedError{"instance_qemu", dpath}
	}

	if !DiskDrivers.HotPluggable(d.Driver) {
		return fmt.Errorf("Unknown hotpuggable disk driver: %s", d.Driver)
	}

	if d.IsLocal() {
		devPath := filepath.Join(CHROOTDIR, r.name, d.Path)
		if err := os.Remove(devPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	// Remove from the guest
	switch d.Driver {
	case "virtio-blk-pci":
		ts := time.Now()
		if err := r.mon.Run(qmp.Command{"device_del", &qt.StrID{d.QdevID()}}, nil); err != nil {
			return fmt.Errorf("device_del error: %s", err)
		}

		// ... and wait until the operation is completed
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		switch _, err := r.mon.WaitDeviceDeletedEvent(ctx, d.QdevID(), uint64(ts.Unix())); {
		case err == nil:
		case err == context.DeadlineExceeded:
			return fmt.Errorf("device_del timeout error: failed to complete within 60 seconds")
		default:
			return err
		}
	case "scsi-hd":
		if err := r.mon.Run(qmp.Command{"device_del", &qt.StrID{d.QdevID()}}, nil); err != nil {
			return fmt.Errorf("device_del error: %s", err)
		}
	}

	blkDevs := make([]qt.BlockInfo, 0, 8)
	if err := r.mon.Run(qmp.Command{"query-block", nil}, &blkDevs); err != nil {
		return err
	}

	for _, dev := range blkDevs {
		if dev.Device == d.BaseName() {
			// That's a fucking shame that we need to do it exactly that way
			if _, err := r.mon.RunHuman("drive_del " + d.BaseName()); err != nil {
				return fmt.Errorf("drive_del error: %s", err)
			}
		}
	}

	if err := r.Disks.Remove(d.Path); err != nil {
		return err
	}

	return nil
}

func (r *InstanceQemu) SetDiskReadIops(dpath string, iops int) error {
	if iops < 0 {
		return fmt.Errorf("Invalid iops value: cannot be less than 0")
	}

	d := r.Disks.Get(dpath)
	if d == nil {
		return &NotConnectedError{"instance_qemu", dpath}
	}

	opts := qt.BlockIOThrottle{
		Device: d.BaseName(),
		IopsRd: iops,
		IopsWr: d.IopsWr,
	}

	if err := r.mon.Run(qmp.Command{"block_set_io_throttle", &opts}, nil); err != nil {
		return err
	}

	d.IopsRd = iops

	return nil
}

func (r *InstanceQemu) SetDiskWriteIops(dpath string, iops int) error {
	if iops < 0 {
		return fmt.Errorf("Invalid iops value: cannot be less than 0")
	}

	d := r.Disks.Get(dpath)
	if d == nil {
		return &NotConnectedError{"instance_qemu", dpath}
	}

	opts := qt.BlockIOThrottle{
		Device: d.BaseName(),
		IopsRd: d.IopsRd,
		IopsWr: iops,
	}

	if err := r.mon.Run(qmp.Command{"block_set_io_throttle", &opts}, nil); err != nil {
		return err
	}

	d.IopsWr = iops

	return nil
}

func (r *InstanceQemu) RemoveDiskBitmap(dpath string) error {
	d := r.Disks.Get(dpath)
	if d == nil {
		return &NotConnectedError{"instance_qemu", dpath}
	}

	if !d.HasBitmap {
		return nil
	}

	opts := qt.BlockDirtyBitmapOptions{
		Node: d.BaseName(),
		Name: "backup",
	}

	return r.mon.Run(qmp.Command{"block-dirty-bitmap-remove", &opts}, nil)

}

func (r *InstanceQemu) initNetwork() error {
	pciDevs := make([]qt.PCIInfo, 0, 1)
	if err := r.mon.Run(qmp.Command{"query-pci", nil}, &pciDevs); err != nil {
		return err
	}

	pool := make(NetIfaces, 0, 8)
	for _, dev := range pciDevs[0].Devices {
		// {'class': 512, 'desc': 'Ethernet controller'}
		if dev.ClassInfo.Class != 512 {
			continue
		}

		netif := NetIface{Addr: fmt.Sprintf("0x%x", dev.Slot)}

		if err := r.mon.Run(qmp.Command{"qom-get", &qt.QomQuery{dev.QdevID, "type"}}, &netif.Driver); err != nil {
			return err
		}
		if !NetDrivers.Exists(netif.Driver) {
			continue
		}

		if err := r.mon.Run(qmp.Command{"qom-get", &qt.QomQuery{dev.QdevID, "mac"}}, &netif.HwAddr); err != nil {
			return err
		}

		// QdevId -- is a string with prefix 'net_'
		// E.g: net_alice
		netif.Ifname = dev.QdevID[4:]

		// Ifup, Ifdown
		scripts := struct {
			Ifup   string `json:"ifup"`
			Ifdown string `json:"ifdown"`
		}{}
		c, err := ioutil.ReadFile(filepath.Join(CHROOTDIR, r.name, "run/net", netif.Ifname))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(c, &scripts); err != nil {
			return err
		}
		netif.Ifup = scripts.Ifup
		netif.Ifdown = scripts.Ifdown

		pool = append(pool, netif)
	}

	r.NetIfaces = pool

	return nil
}

func (r InstanceQemu) GetNetIfaces() NetIfaces {
	nn := make(NetIfaces, len(r.NetIfaces))
	copy(nn, r.NetIfaces)
	return nn
}

func (r *InstanceQemu) AppendNetIface(iface NetIface) error {
	if r.NetIfaces.Exists(iface.Ifname) {
		return &AlreadyConnectedError{"instance_qemu", iface.Ifname}
	}

	if !NetDrivers.HotPluggable(iface.Driver) {
		return fmt.Errorf("Unknown hotpuggable network interface driver: %s", iface.Driver)
	}

	hostOpts := qt.NetdevTapOptions{
		Type:       "tap",
		ID:         iface.Ifname,
		Ifname:     iface.Ifname,
		Vhost:      "on",
		Script:     "no",
		Downscript: "no",
	}
	opts := qt.DeviceOptions{
		Driver: iface.Driver,
		Netdev: iface.Ifname,
		Id:     iface.QdevID(),
		Mac:    iface.HwAddr,
	}

	if err := AddTapInterface(iface.Ifname, r.uid); err != nil {
		return err
	}
	if err := SetInterfaceUp(iface.Ifname); err != nil {
		return err
	}
	if err := r.mon.Run(qmp.Command{"netdev_add", &hostOpts}, nil); err != nil {
		return err
	}
	if err := r.mon.Run(qmp.Command{"device_add", &opts}, nil); err != nil {
		return err
	}

	b, err := json.Marshal(iface)
	if err != nil {
		return err
	}
	ifaceConf := filepath.Join(CHROOTDIR, r.name, "run/net", iface.Ifname)
	if err := ioutil.WriteFile(ifaceConf, b, 0644); err != nil {
		return err
	}

	r.NetIfaces.Append(&iface)

	return nil
}

func (r *InstanceQemu) RemoveNetIface(ifname string) error {
	iface := r.NetIfaces.Get(ifname)
	if iface == nil {
		return &NotConnectedError{"instance_qemu", ifname}
	}

	if !NetDrivers.HotPluggable(iface.Driver) {
		return fmt.Errorf("Unknown hotpuggable network interface driver: %s", iface.Driver)
	}

	// Remove from the guest and wait until the operation is completed
	ts := time.Now()
	if err := r.mon.Run(qmp.Command{"device_del", &qt.StrID{iface.QdevID()}}, nil); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	switch _, err := r.mon.WaitDeviceDeletedEvent(ctx, iface.QdevID(), uint64(ts.Unix())); {
	case err == nil:
	case err == context.DeadlineExceeded:
		return fmt.Errorf("device_del timeout error: failed to complete within 60 seconds")
	default:
		return err
	}

	// Remove the backend
	if err := r.mon.Run(qmp.Command{"netdev_del", &qt.StrID{iface.Ifname}}, nil); err != nil {
		return err
	}

	if err := r.NetIfaces.Remove(iface.Ifname); err != nil {
		return err
	}

	if err := DelTapInterface(iface.Ifname); err != nil {
		return fmt.Errorf("Cannot remove the tap interface:", err)
	}

	ifaceConf := filepath.Join(CHROOTDIR, r.name, "run/net", iface.Ifname)
	if err := os.Remove(ifaceConf); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (r *InstanceQemu) SetNetIfaceUpScript(ifname, scriptPath string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetNetIfaceDownScript(ifname, scriptPath string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetNetIfaceLinkUp(ifname string) error {
	iface := r.NetIfaces.Get(ifname)
	if iface == nil {
		return &NotConnectedError{"instance_qemu", ifname}
	}

	linkState := struct {
		Name    string `json:"name"`
		Carrier bool   `json:"up"`
	}{
		iface.QdevID(),
		true,
	}

	return r.mon.Run(qmp.Command{"set_link", &linkState}, nil)
}

func (r *InstanceQemu) SetNetIfaceLinkDown(ifname string) error {
	iface := r.NetIfaces.Get(ifname)
	if iface == nil {
		return &NotConnectedError{"instance_qemu", ifname}
	}

	linkState := struct {
		Name    string `json:"name"`
		Carrier bool   `json:"up"`
	}{
		iface.QdevID(),
		false,
	}

	return r.mon.Run(qmp.Command{"set_link", &linkState}, nil)
}

func (r *InstanceQemu) initChannels() error {
	charDevs := make([]qt.ChardevInfo, 0, 1)
	if err := r.mon.Run(qmp.Command{"query-chardev", nil}, &charDevs); err != nil {
		return err
	}

	pool := make(Channels, 0, 1)
	for _, dev := range charDevs {
		if !strings.HasPrefix(dev.Label, "c_") {
			continue
		}

		ch := VirtioChannel{
			// Label -- is a string with prefix 'c_'
			ID: dev.Label[2:],
		}

		if err := r.mon.Run(qmp.Command{"qom-get", &qt.QomQuery{ch.QdevID(), "name"}}, &ch.Name); err != nil {
			continue
		}

		var nr int
		if err := r.mon.Run(qmp.Command{"qom-get", &qt.QomQuery{ch.QdevID(), "nr"}}, &nr); err != nil {
			return err
		}
		ch.Addr = fmt.Sprintf("0x%.2x", nr)

		pool = append(pool, ch)
	}

	r.Channels = pool

	return nil
}

func (r InstanceQemu) GetChannels() Channels {
	cc := make(Channels, len(r.Channels))
	copy(cc, r.Channels)
	return cc
}

func (r *InstanceQemu) AppendChannel(ch VirtioChannel) error {
	if r.Channels.Exists(ch.ID) {
		return &AlreadyConnectedError{"instance_qemu", ch.ID}
	}
	if r.Channels.NameExists(ch.Name) {
		return &AlreadyConnectedError{"instance_qemu", ch.Name}
	}

	chardevOpts := qt.ChardevOptions{
		ID: ch.CharDevName(),
		Backend: qt.ChardevBackend{
			Type: "socket",
			Data: qt.ChardevSocket{
				Addr: qt.SocketAddressLegacy{
					Type: "unix",
					Data: qt.UnixSocketAddress{
						Path: fmt.Sprintf("%s.%s", filepath.Join(QMPMONDIR, r.name), ch.ID),
					},
				},
				Server: true,
				Wait:   false,
			},
		},
	}

	devOpts := qt.DeviceOptions{
		Driver:  "virtserialport",
		Chardev: ch.CharDevName(),
		Id:      ch.QdevID(),
		Name:    ch.Name,
	}

	if err := r.mon.Run(qmp.Command{"chardev-add", &chardevOpts}, nil); err != nil {
		return err
	}
	if err := r.mon.Run(qmp.Command{"device_add", &devOpts}, nil); err != nil {
		return err
	}

	r.Channels.Append(&ch)

	sock := filepath.Join(CHROOTDIR, r.name, QMPMONDIR, fmt.Sprintf("%s.%s", r.name, ch.ID))
	link := filepath.Join(QMPMONDIR, fmt.Sprintf("%s.%s", r.name, ch.ID))

	if err := os.Symlink(sock, link); err != nil && !os.IsExist(err) {
		return err
	}

	return nil
}

func (r *InstanceQemu) RemoveChannel(id string) error {
	ch := r.Channels.Get(id)
	if ch == nil {
		return &NotConnectedError{"instance_qemu", id}
	}

	if err := r.mon.Run(qmp.Command{"device_del", &qt.StrID{ch.QdevID()}}, nil); err != nil {
		return err
	}
	if err := r.mon.Run(qmp.Command{"chardev-remove", &qt.StrID{ch.CharDevName()}}, nil); err != nil {
		return err
	}

	if err := r.Channels.Remove(ch.ID); err != nil {
		return err
	}

	sock := filepath.Join(CHROOTDIR, r.name, QMPMONDIR, fmt.Sprintf("%s.%s", r.name, ch.ID))
	link := filepath.Join(QMPMONDIR, fmt.Sprintf("%s.%s", r.name, ch.ID))

	for _, f := range []string{sock, link} {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func (r *InstanceQemu) initKern() error {
	r.Kernel.Image = r.startupCfg.GetKernelImage()
	r.Kernel.Initrd = r.startupCfg.GetKernelInitrd()
	r.Kernel.Cmdline = r.startupCfg.GetKernelCmdline()
	r.Kernel.Modiso = r.startupCfg.GetKernelModiso()

	return nil
}

func (r InstanceQemu) GetKernelImage() string {
	return r.Kernel.Image
}

func (r InstanceQemu) GetKernelCmdline() string {
	return r.Kernel.Cmdline
}

func (r InstanceQemu) GetKernelInitrd() string {
	return r.Kernel.Initrd
}

func (r InstanceQemu) GetKernelModiso() string {
	return r.Kernel.Modiso
}

func (r *InstanceQemu) RemoveKernelConf() error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetKernelImage(s string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetKernelCmdline(s string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetKernelInitrd(s string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetKernelModiso(s string) error {
	return ErrNotImplemented
}

func (r InstanceQemu) SetVNCPassword(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("Invalid password string")
	}

	opts := struct {
		Password string `json:"password"`
	}{
		Password: s,
	}

	if err := r.mon.Run(qmp.Command{"change-vnc-password", opts}, nil); err != nil {
		return err
	}

	return nil
}
