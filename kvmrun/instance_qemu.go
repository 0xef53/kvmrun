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
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	qemu_types "github.com/0xef53/kvmrun/internal/qemu/types"
	"github.com/0xef53/kvmrun/kvmrun/backend/block"

	cg "github.com/0xef53/go-cgroups"
	qmp "github.com/0xef53/go-qmp/v2"
	"golang.org/x/sync/errgroup"
)

// InstanceQemu represents a configuration of a running QEMU instance.
type InstanceQemu struct {
	*InstanceProperties

	mon         *qmp.Monitor `json:"-"`
	startupConf Instance     `json:"-"`
	pid         int          `json:"-"`
	qemuVer     QemuVersion  `json:"-"`

	scsiBuses map[string]*SCSIBusInfo `json:"-"`
}

func GetInstanceQemu(vmname string, mon *qmp.Monitor) (Instance, error) {
	if mon == nil {
		return nil, &NotRunningError{vmname}
	}

	inner := InstanceQemu{
		InstanceProperties: &InstanceProperties{
			name: vmname,
		},
		mon: mon,
	}

	if u, err := user.Lookup(vmname); err == nil {
		if uid, err := strconv.Atoi(u.Uid); err == nil {
			inner.uid = uid
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}

	switch err := inner.initMachine(); {
	case err == nil:
	case qmp.IsSocketNotAvailable(err), qmp.IsSocketClosed(err):
		return nil, &NotRunningError{vmname}
	default:
		return nil, err
	}

	if c, err := GetStartupConf(vmname); err == nil {
		inner.startupConf = c
	} else {
		return nil, fmt.Errorf("unable to load startup config: %s", err)
	}

	var vmi Instance

	switch t := inner.GetMachineType(); t.Chipset {
	case QEMU_CHIPSET_I440FX:
		vmi = &InstanceQemu_i440fx{InstanceQemu: &inner}
	default:
		return nil, fmt.Errorf("unsupported machine type: %s", t)
	}

	var gr errgroup.Group

	gr.Go(func() error { return inner.initFirmware() })
	gr.Go(func() error { return inner.initCPU() })
	gr.Go(func() error { return inner.initMemory() })
	gr.Go(func() error { return inner.initHostPCIDevices() })
	gr.Go(func() error { return inner.initInputDevices() })
	gr.Go(func() error { return inner.initVSockDevice() })
	gr.Go(func() error { return inner.initCloudInitDrive() })
	gr.Go(func() error { return inner.initKern() })
	gr.Go(func() error { return inner.initProxyServers() })

	if x, ok := vmi.(interface{ init() error }); ok {
		gr.Go(func() error { return x.init() })
	} else {
		return nil, fmt.Errorf("invalid machine interface: %s", inner.MachineType)
	}

	switch err := gr.Wait(); {
	case err == nil:
	case qmp.IsSocketNotAvailable(err), qmp.IsSocketClosed(err):
		return nil, &NotRunningError{vmname}
	default:
		return nil, err
	}

	return vmi, nil
}

func (r InstanceQemu) IsIncoming() bool {
	return false
}

func (r InstanceQemu) Save() error {
	return ErrNotImplemented
}

func (r InstanceQemu) Status() (InstanceState, error) {
	var status InstanceState

	// Incoming migration
	var st qemu_types.StatusInfo
	if err := r.mon.Run(qmp.Command{"query-status", nil}, &st); err != nil {
		return StateNoState, err
	}

	switch st.Status {
	case "inmigrate", "postmigrate", "finish-migrate":
		status = StateIncoming
	case "running":
		status = StateRunning
	case "paused":
		status = StatePaused
	}

	// Outgoing migration
	migrSt := struct {
		Status    string `json:"status"`
		TotalTime uint64 `json:"total-time"`
	}{}

	if err := r.mon.Run(qmp.Command{"query-migrate", nil}, &migrSt); err != nil {
		return StateNoState, err
	}

	switch migrSt.Status {
	case "setup", "cancelling", "active", "postcopy-active", "pre-switchover", "device":
		status = StateMigrating
	case "completed":
		if migrSt.TotalTime != 0 {
			status = StateMigrated
		}
	}

	// Return inmigrate also if there is at least one disk with an active migration job
	jobs := make([]qemu_types.BlockJobInfo, 0, len(r.Disks))
	if err := r.mon.Run(qmp.Command{"query-block-jobs", nil}, &jobs); err != nil {
		return StateNoState, err
	}
	for _, j := range jobs {
		if strings.HasPrefix(j.Device, "migr_") {
			status = StateMigrating
		}
	}

	return status, nil
}

func (r *InstanceQemu) initMachine() error {
	var mtype string

	if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{"/machine", "type"}}, &mtype); err != nil {
		return err
	}

	r.MachineType = strings.TrimSuffix(strings.ToLower(mtype), "-machine")

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

func (r *InstanceQemu) initFirmware() error {
	r.Firmware.Image = r.startupConf.GetFirmwareImage()

	if fwflash := r.startupConf.GetFirmwareFlash(); fwflash != nil {
		r.Firmware.Flash = fwflash.Path
		r.Firmware.flashDisk = fwflash
	}

	return nil
}

func (r InstanceQemu) Pid() int {
	return r.pid
}

func (r InstanceQemu) GetQemuVersion() QemuVersion {
	return r.qemuVer
}

func (r *InstanceQemu) initCPU() error {
	var firstThreadID int

	// Actual vCPU count
	switch {
	case r.qemuVer < 21200:
		attachedCPUs := make([]qemu_types.CPUInfo, 0, 8)
		if err := r.mon.Run(qmp.Command{"query-cpus", nil}, &attachedCPUs); err != nil {
			return err
		}
		r.CPU.Actual = len(attachedCPUs)
		firstThreadID = attachedCPUs[0].ThreadID
	default:
		attachedCPUs := make([]qemu_types.CPUInfoFast, 0, 8)
		if err := r.mon.Run(qmp.Command{"query-cpus-fast", nil}, &attachedCPUs); err != nil {
			return err
		}
		r.CPU.Actual = len(attachedCPUs)
		firstThreadID = attachedCPUs[0].ThreadID
	}

	// Total vCPU count
	r.CPU.Total = r.startupConf.GetTotalCPUs()

	// Number of processor sockets
	r.CPU.Sockets = r.startupConf.GetCPUSockets()

	// CPU model
	r.CPU.Model = r.startupConf.GetCPUModel()

	// Process ID
	pid, err := func() (int, error) {
		fd, err := os.Open(filepath.Join("/proc", fmt.Sprintf("%d", firstThreadID), "status"))
		if err != nil {
			return 0, err
		}
		defer fd.Close()

		scanner := bufio.NewScanner(fd)

		for scanner.Scan() {
			if b := scanner.Bytes(); bytes.HasPrefix(b, []byte("Tgid:")) {
				bf := bytes.Fields(b)
				if len(bf) < 2 {
					continue
				}

				return strconv.Atoi(string(bf[1]))
			}
		}

		return 0, scanner.Err()
	}()
	if err != nil {
		return err
	}

	r.pid = pid

	// Cgroups CPU quota
	if g, err := cg.LookupCgroupByPid(r.pid, "cpu"); err == nil {
		wantSuffix := filepath.Join(CGROOTPATH, r.name)
		if strings.HasSuffix(g.GetPath(), wantSuffix) {
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

func (r *InstanceQemu) SetActualCPUs(n int) error {
	if n < 1 {
		return fmt.Errorf("invalid cpu count: cannot be less than 1")
	}
	if n > r.CPU.Total {
		return fmt.Errorf("invalid actual cpu: cannot be large than total cpu (%d)", r.CPU.Total)
	}

	availableCPUs := make([]qemu_types.HotpluggableCPU, 0, 8)
	if err := r.mon.Run(qmp.Command{"query-hotpluggable-cpus", nil}, &availableCPUs); err != nil {
		return err
	}

	sort.SliceStable(availableCPUs, func(i, j int) bool {
		if availableCPUs[i].Props.SocketID < availableCPUs[j].Props.SocketID {
			return true
		}
		if availableCPUs[i].Props.SocketID > availableCPUs[j].Props.SocketID {
			return false
		}
		if availableCPUs[i].Props.CoreID < availableCPUs[j].Props.CoreID {
			return true
		}
		return false
	})

	// This slice cannot be empty
	// because a virtual machine always has at least one core
	cpuType := availableCPUs[0].Type

	switch {
	case n < r.CPU.Actual:
		// Decrease
		if r.qemuVer < 20700 {
			return fmt.Errorf("hot-unplug operation is not supported in QEMU %s", r.qemuVer)
		}

		for idx := len(availableCPUs) - 1; idx >= 0; idx-- {
			if len(availableCPUs[idx].QomPath) == 0 {
				continue
			}
			if r.CPU.Actual == n {
				break
			}
			if err := r.mon.Run(qmp.Command{"device_del", qemu_types.StrID{availableCPUs[idx].QomPath}}, nil); err != nil {
				return err
			}
			r.CPU.Actual--
		}
	case n > r.CPU.Actual:
		// Increase
		for _, cpu := range availableCPUs {
			if len(cpu.QomPath) != 0 {
				continue
			}
			if r.CPU.Actual == n {
				break
			}
			opts := qemu_types.CPUDeviceOptions{
				Driver:   cpuType,
				SocketID: cpu.Props.SocketID,
				CoreID:   cpu.Props.CoreID,
				ThreadID: 0,
			}
			if err := r.mon.Run(qmp.Command{"device_add", &opts}, nil); err != nil {
				return err
			}
			r.CPU.Actual++
		}
	}

	return nil
}

func (r *InstanceQemu) SetTotalCPUs(n int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetCPUSockets(n int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetCPUModel(model string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetCPUQuota(quota int) error {
	relpath := filepath.Join(CGROOTPATH, r.name)

	cpuGroup, err := cg.NewCpuGroup(relpath, r.pid)
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

func (r InstanceQemu) SetMachineType(_ string) error {
	return ErrNotImplemented
}

func (r InstanceQemu) SetFirmwareImage(_ string) error {
	return ErrNotImplemented
}

func (r InstanceQemu) SetFirmwareFlash(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) RemoveFirmwareConf() error {
	return ErrNotImplemented
}

func (r *InstanceQemu) initMemory() error {
	balloonInfo := struct {
		Actual uint64 `json:"actual"`
	}{}

	if err := r.mon.Run(qmp.Command{"query-balloon", nil}, &balloonInfo); err == nil {
		r.Mem.Actual = int(balloonInfo.Actual >> 20)
	} else {
		if _, ok := err.(*qmp.DeviceNotActive); ok {
			r.Mem.Actual = r.startupConf.GetActualMem()
		} else {
			return err
		}
	}

	r.Mem.Total = r.startupConf.GetTotalMem()

	return nil
}

func (r *InstanceQemu) SetActualMem(s int) error {
	if s < 1 {
		return fmt.Errorf("invalid memory size: cannot be less than 1")
	}
	if s > r.Mem.Total {
		return fmt.Errorf("invalid actual memory: cannot be large than total memory (%d)", r.Mem.Total)
	}

	if err := r.mon.Run(qmp.Command{"balloon", qemu_types.IntValue{s << 20}}, nil); err != nil {
		return err
	}

	r.Mem.Actual = s

	return nil
}

func (r *InstanceQemu) SetTotalMem(_ int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) initHostPCIDevices() error {
	r.HostPCIDevices = r.startupConf.GetHostPCIDevices()

	return nil
}

func (r *InstanceQemu) AppendHostPCI(_ HostPCI) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) RemoveHostPCI(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetHostPCIMultifunctionOption(_ string, _ bool) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetHostPCIPrimaryGPUOption(_ string, _ bool) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) initInputDevices() error {
	r.Inputs = r.startupConf.GetInputDevices()

	return nil
}

func (r *InstanceQemu) AppendInputDevice(_ InputDevice) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) RemoveInputDevice(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) AppendCdrom(_ Cdrom) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) InsertCdrom(_ Cdrom, _ int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) RemoveCdrom(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) ChangeCdromMedia(name, media string) error {
	d := r.Cdroms.Get(name)
	if d == nil {
		return &NotConnectedError{"instance_qemu", name}
	}

	if d.Media == media {
		return &AlreadyConnectedError{"instance_qemu", media}
	}

	var success bool

	var oldMedia string = d.Media
	var oldMediaLocal bool

	if _, ok := d.Backend.(*block.Device); ok {
		oldMediaLocal = true
	}

	defer func() {
		if oldMediaLocal && success {
			os.Remove(filepath.Join(CHROOTDIR, r.name, oldMedia))
		}
	}()

	if b, err := NewCdromBackend(media); err == nil {
		d.Backend = b
	} else {
		return err
	}

	d.Media = media

	if _, ok := d.Backend.(*block.Device); ok {
		devpath := filepath.Join(CHROOTDIR, r.name, d.Media)

		defer func() {
			if !success {
				os.Remove(devpath)
			}
		}()

		stat := syscall.Stat_t{}
		if err := syscall.Stat(d.Media, &stat); err != nil {
			return err
		}

		os.MkdirAll(filepath.Dir(devpath), 0755)

		if err := syscall.Mknod(devpath, syscall.S_IFBLK|uint32(os.FileMode(01640)), int(stat.Rdev)); err != nil {
			if os.IsExist(err) {
				return fmt.Errorf("device is already in use: %s", d.Media)
			}
			return err
		}
		if err := os.Chown(devpath, r.uid, 0); err != nil {
			return err
		}
	}

	//
	// Change in QEMU
	//

	wait := func(fn func(context.Context, string, uint64) (*qmp.Event, error), after time.Time) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		_, err := fn(ctx, d.QdevID(), uint64(after.Unix()))
		if err != nil {
			if err == context.DeadlineExceeded {
				return fmt.Errorf("change medium timeout error: failed to complete within 60 seconds")
			}
			return err
		}

		return nil
	}

	var ts time.Time

	// Open the tray
	ts = time.Now()

	if err := r.mon.Run(qmp.Command{"blockdev-open-tray", &qemu_types.StrID{d.QdevID()}}, nil); err != nil {
		return err
	}

	// ... wait until the tray is opened
	if err := wait(r.mon.WaitDeviceTrayOpenedEvent, ts); err != nil {
		return err
	}

	// Replace the media and close the tray
	opts := struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
	}{
		ID:       d.QdevID(),
		Filename: media,
	}

	ts = time.Now()

	if err := r.mon.Run(qmp.Command{"blockdev-change-medium", &opts}, nil); err != nil {
		return err
	}

	// ... wait until the tray is closed
	if err := wait(r.mon.WaitDeviceTrayClosedEvent, ts); err != nil {
		return err
	}

	success = true

	return nil
}

func (r *InstanceQemu) AppendDisk(_ Disk) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) RemoveDisk(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) initProxyServers() error {
	b, err := ioutil.ReadFile(filepath.Join(CHROOTDIR, r.name, "run/backend_proxy"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(b, &r.Proxy)
}

func (r *InstanceQemu) AppendProxy(proxy Proxy) error {
	if r.Proxy.Exists(proxy.Path) {
		return &AlreadyConnectedError{"instance_conf", proxy.Path}
	}

	r.Proxy.Append(&proxy)

	if b, err := json.MarshalIndent(r.Proxy, "", "    "); err == nil {
		if err := ioutil.WriteFile(filepath.Join(CHROOTDIR, r.name, "run/backend_proxy"), b, 0644); err != nil {
			return err
		}
	} else {
		return err
	}

	return nil
}

func (r *InstanceQemu) RemoveProxy(fullpath string) error {
	if !r.Proxy.Exists(fullpath) {
		return &NotConnectedError{"instance_conf", fullpath}
	}

	if err := r.Proxy.Remove(fullpath); err != nil {
		return err
	}

	if b, err := json.MarshalIndent(r.Proxy, "", "    "); err == nil {
		if err := ioutil.WriteFile(filepath.Join(CHROOTDIR, r.name, "run/backend_proxy"), b, 0644); err != nil {
			return err
		}
	} else {
		return err
	}

	return nil
}

func (r InstanceQemu) ResizeQemuBlockdev(dpath string) error {
	d := r.Disks.Get(dpath)
	if d == nil {
		return &NotConnectedError{"instance_qemu", dpath}
	}

	opts := qemu_types.BlockResizeQuery{
		Device: d.BaseName(),
		Size:   1,
	}

	return r.mon.Run(qmp.Command{"block_resize", &opts}, nil)
}

func (r *InstanceQemu) InsertDisk(_ Disk, _ int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetDiskReadIops(dpath string, iops int) error {
	if iops < 0 {
		return fmt.Errorf("invalid iops value: cannot be less than 0")
	}

	d := r.Disks.Get(dpath)
	if d == nil {
		return &NotConnectedError{"instance_qemu", dpath}
	}

	opts := qemu_types.BlockIOThrottle{
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
		return fmt.Errorf("invalid iops value: cannot be less than 0")
	}

	d := r.Disks.Get(dpath)
	if d == nil {
		return &NotConnectedError{"instance_qemu", dpath}
	}

	opts := qemu_types.BlockIOThrottle{
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

	opts := qemu_types.BlockDirtyBitmapOptions{
		Node: d.BaseName(),
		Name: "backup",
	}

	return r.mon.Run(qmp.Command{"block-dirty-bitmap-remove", &opts}, nil)
}

func (r *InstanceQemu) AppendNetIface(_ NetIface) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) RemoveNetIface(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetNetIfaceQueues(_ string, _ int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetNetIfaceUpScript(_, _ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) SetNetIfaceDownScript(_, _ string) error {
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

func (r *InstanceQemu) initVSockDevice() error {
	vsock := VirtioVSock{}

	if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{"vsock_device", "guest-cid"}}, &vsock.ContextID); err != nil {
		if _, ok := err.(*qmp.DeviceNotFound); ok {
			return nil
		}
		return err
	}

	// An addr/slot on the PCI bus
	var pciAddr string
	if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{"vsock_device", "legacy-addr"}}, &pciAddr); err == nil {
		vsock.Addr = fmt.Sprintf("0x%s", strings.Split(pciAddr, ".")[0])
	}

	r.VSockDevice = &vsock

	return nil
}

func (r *InstanceQemu) AppendVSockDevice(cid uint32) error {
	if r.VSockDevice != nil {
		return &AlreadyConnectedError{"instance_qemu", "vsock device"}
	}

	vsock := new(VirtioVSock)

	switch {
	case cid == 0:
		vsock.Auto = true
		vsock.ContextID = uint32(r.pid)
	case cid >= 3:
		vsock.ContextID = cid
	default:
		return ErrIncorrectContextID
	}

	devOpts := qemu_types.DeviceOptions{
		Driver:  "vhost-vsock-pci",
		Id:      "vsock_device",
		GuestID: vsock.ContextID,
	}

	if err := r.mon.Run(qmp.Command{"device_add", &devOpts}, nil); err != nil {
		return err
	}

	r.VSockDevice = vsock

	return nil
}

func (r *InstanceQemu) RemoveVSockDevice() error {
	if r.VSockDevice == nil {
		return &NotConnectedError{"instance_conf", "vsock device"}
	}

	if err := r.mon.Run(qmp.Command{"device_del", &qemu_types.StrID{"vsock_device"}}, nil); err != nil {
		return err
	}

	r.VSockDevice = nil

	return nil
}

func (r *InstanceQemu) initCloudInitDrive() error {
	r.CIDrive.Path = r.startupConf.GetCloudInitDrive()

	return nil
}

func (r *InstanceQemu) SetCloudInitDrive(s string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) initKern() error {
	r.Kernel.Image = r.startupConf.GetKernelImage()
	r.Kernel.Initrd = r.startupConf.GetKernelInitrd()
	r.Kernel.Cmdline = r.startupConf.GetKernelCmdline()
	r.Kernel.Modiso = r.startupConf.GetKernelModiso()

	return nil
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
		return fmt.Errorf("invalid password string")
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
