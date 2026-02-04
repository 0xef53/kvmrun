package kvmrun

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	cg "github.com/0xef53/kvmrun/internal/cgroups"
	qemu_types "github.com/0xef53/kvmrun/internal/qemu/types"
	"github.com/0xef53/kvmrun/internal/version"
	"github.com/0xef53/kvmrun/kvmrun/backend"
	"github.com/0xef53/kvmrun/kvmrun/backend/block"
	"github.com/0xef53/kvmrun/kvmrun/backend/file"

	qmp "github.com/0xef53/go-qmp/v2"
	"golang.org/x/sync/errgroup"
)

// InstanceQemu represents a configuration of a running QEMU instance.
type InstanceQemu struct {
	*InstanceProperties

	mon         *qmp.Monitor     `json:"-"`
	startupConf Instance         `json:"-"`
	pid         int              `json:"-"`
	qemuVer     *version.Version `json:"-"`

	scsiBuses map[string]*SCSIBusInfo `json:"-"`
}

func GetInstanceQemu(vmname string, mon *qmp.Monitor) (Instance, error) {
	vmname = strings.TrimSpace(vmname)

	if len(vmname) == 0 {
		return nil, fmt.Errorf("empty machine name")
	}

	if mon == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotRunning, vmname)
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
		return nil, fmt.Errorf("%w: %s", ErrNotRunning, vmname)
	default:
		return nil, err
	}

	if c, err := GetStartupConf(vmname); err == nil {
		inner.startupConf = c
	} else {
		return nil, fmt.Errorf("unable to load startup config: %s", err)
	}

	var vmi Instance

	switch t := inner.MachineTypeGet(); t.Chipset {
	case QEMU_CHIPSET_I440FX:
		vmi = &InstanceQemu_i440fx{InstanceQemu: &inner}
	default:
		return nil, fmt.Errorf("unsupported machine type: %s", t)
	}

	var gr errgroup.Group

	gr.Go(func() error { return inner.initFirmware() })
	gr.Go(func() error { return inner.initCPU() })
	gr.Go(func() error { return inner.initMemory() })
	gr.Go(func() error { return inner.initHostDevicePool() })
	gr.Go(func() error { return inner.initInputDevicePool() })
	gr.Go(func() error { return inner.initVSockDevice() })
	gr.Go(func() error { return inner.initExtKernel() })

	if x, ok := vmi.(interface{ init() error }); ok {
		gr.Go(func() error { return x.init() })
	} else {
		return nil, fmt.Errorf("invalid machine interface: %s", inner.MachineType)
	}

	switch err := gr.Wait(); {
	case err == nil:
	case qmp.IsSocketNotAvailable(err), qmp.IsSocketClosed(err):
		return nil, fmt.Errorf("%w: %s", ErrNotRunning, vmname)
	default:
		return nil, err
	}

	return vmi, nil
}

func (r InstanceQemu) PID() int {
	return r.pid
}

func (r InstanceQemu) QemuVersion() *version.Version {
	return r.qemuVer
}

func (r InstanceQemu) IsIncoming() bool {
	return false
}

func (r InstanceQemu) Save() error {
	return ErrNotImplemented
}

func (r InstanceQemu) Status() (InstanceState, error) {

	var st qemu_types.StatusInfo

	if err := r.mon.Run(qmp.Command{Name: "query-status", Arguments: nil}, &st); err != nil {
		return StateNoState, err
	}

	var status InstanceState

	switch st.Status {
	case "inmigrate", "postmigrate", "finish-migrate":
		status = StateIncoming
	case "running":
		status = StateRunning
	case "paused":
		status = StatePaused
	}

	// Check if outgoing migration is in progress
	migrSt := struct {
		Status    string `json:"status"`
		TotalTime uint64 `json:"total-time"`
	}{}

	if err := r.mon.Run(qmp.Command{Name: "query-migrate", Arguments: nil}, &migrSt); err != nil {
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
	jobs := make([]qemu_types.BlockJobInfo, 0, r.Disks.Len())

	if err := r.mon.Run(qmp.Command{Name: "query-block-jobs", Arguments: nil}, &jobs); err != nil {
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

	qomQuery := qemu_types.QomQuery{Path: "/machine", Property: "type"}

	if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &qomQuery}, &mtype); err != nil {
		return err
	}

	r.MachineType = strings.TrimSuffix(strings.ToLower(mtype), "-machine")

	ver := struct {
		Qemu *version.Version `json:"qemu"`
	}{}

	if err := r.mon.Run(qmp.Command{Name: "query-version", Arguments: nil}, &ver); err != nil {
		return err
	}

	// E.g.: 21101 is 2.11.1
	r.qemuVer = ver.Qemu

	return nil
}

func (r InstanceQemu) MachineTypeSet(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) initFirmware() error {
	r.Firmware = r.startupConf.FirmwareGet()

	return nil
}

func (r InstanceQemu) FirmwareSetImage(_ string) error {
	return ErrNotImplemented
}

func (r InstanceQemu) FirmwareSetFlash(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) FirmwareRemoveConf() error {
	return ErrNotImplemented
}

func (r *InstanceQemu) initMemory() error {
	balloonInfo := struct {
		Actual uint64 `json:"actual"`
	}{}

	if err := r.mon.Run(qmp.Command{Name: "query-balloon", Arguments: nil}, &balloonInfo); err == nil {
		r.Memory.Actual = int(balloonInfo.Actual >> 20)
	} else {
		if _, ok := err.(*qmp.DeviceNotActive); ok {
			r.Memory.Actual = r.startupConf.MemoryGetActual()
		} else {
			return err
		}
	}

	r.Memory.Total = r.startupConf.MemoryGetTotal()

	return nil
}

func (r *InstanceQemu) MemorySetActual(value int) (err error) {
	prev := r.Memory.Actual

	defer func() {
		if err != nil {
			r.Memory.Actual = prev
		}
	}()

	err = r.Memory.SetActual(value)

	if err == nil {
		err = r.mon.Run(qmp.Command{Name: "balloon", Arguments: qemu_types.IntValue{Value: value << 20}}, nil)
	}

	return err
}

func (r *InstanceQemu) MemorySetTotal(_ int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) initCPU() error {
	var firstThreadID int

	// Actual vCPU count
	switch {
	case r.qemuVer.Int() < 21200:
		attachedCPUs := make([]qemu_types.CPUInfo, 0, 8)

		if err := r.mon.Run(qmp.Command{Name: "query-cpus", Arguments: nil}, &attachedCPUs); err != nil {
			return err
		}

		r.CPU.Actual = len(attachedCPUs)

		firstThreadID = attachedCPUs[0].ThreadID
	default:
		attachedCPUs := make([]qemu_types.CPUInfoFast, 0, 8)

		if err := r.mon.Run(qmp.Command{Name: "query-cpus-fast", Arguments: nil}, &attachedCPUs); err != nil {
			return err
		}

		r.CPU.Actual = len(attachedCPUs)

		firstThreadID = attachedCPUs[0].ThreadID
	}

	// Total vCPU count
	r.CPU.Total = r.startupConf.CPUGetTotal()

	// Number of sockets
	r.CPU.Sockets = r.startupConf.CPUGetSockets()

	// CPU model
	r.CPU.Model = r.startupConf.CPUGetModel()

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
	if mgr, err := cg.LoadManager(r.pid); err == nil {
		if v, err := mgr.GetCpuQuota(); err == nil {
			r.CPU.Quota = int(v)
		} else {
			return err
		}
	}

	return nil
}

func (r *InstanceQemu) CPUSetActual(value int) error {
	if value < 1 {
		return fmt.Errorf("invalid cpu count: cannot be less than 1")
	}

	if value > r.CPU.Total {
		return fmt.Errorf("invalid actual cpu: cannot be large than total cpu (%d)", r.CPU.Total)
	}

	availableCPUs := make([]qemu_types.HotpluggableCPU, 0, 8)

	if err := r.mon.Run(qmp.Command{Name: "query-hotpluggable-cpus", Arguments: nil}, &availableCPUs); err != nil {
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

	// This slice cannot be empty because a virtual machine always
	// has at least one core
	cpuType := availableCPUs[0].Type

	switch {
	case value < r.CPU.Actual:
		// Decrease
		if r.qemuVer.Int() < 20700 {
			return fmt.Errorf("hot-unplug operation is not supported in QEMU %s", r.qemuVer)
		}

		for idx := len(availableCPUs) - 1; idx >= 0; idx-- {
			if len(availableCPUs[idx].QomPath) == 0 {
				continue
			}

			if r.CPU.Actual == value {
				break
			}

			if err := r.mon.Run(qmp.Command{Name: "device_del", Arguments: qemu_types.StrID{ID: availableCPUs[idx].QomPath}}, nil); err != nil {
				return err
			}

			r.CPU.Actual--
		}
	case value > r.CPU.Actual:
		// Increase
		for _, cpu := range availableCPUs {
			if len(cpu.QomPath) != 0 {
				continue
			}

			if r.CPU.Actual == value {
				break
			}

			opts := qemu_types.CPUDeviceOptions{
				Driver:   cpuType,
				SocketID: cpu.Props.SocketID,
				CoreID:   cpu.Props.CoreID,
				ThreadID: 0,
			}

			if err := r.mon.Run(qmp.Command{Name: "device_add", Arguments: &opts}, nil); err != nil {
				return err
			}

			r.CPU.Actual++
		}
	}

	return nil
}

func (r *InstanceQemu) CPUSetTotal(_ int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) CPUSetSockets(_ int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) CPUSetModel(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) CPUSetQuota(value int) (err error) {
	prev := r.CPU.Quota

	defer func() {
		if err != nil {
			r.CPU.Quota = prev
		}
	}()

	if err := r.CPU.SetQuota(value); err != nil {
		return err
	}

	mgr, err := cg.LoadManager(r.pid)
	if err != nil {
		return err
	}

	return mgr.SetCpuQuota(int64(value))
}

func (r *InstanceQemu) initInputDevicePool() error {
	if c, ok := r.startupConf.(*StartupConf); ok {
		for _, key := range c.InputDevices.Pool.Keys() {
			r.InputDevices.Append(c.InputDevices.Get(key))
		}

		return nil
	}

	return fmt.Errorf("cannot init input-devices")
}

func (r *InstanceQemu) InputDeviceAppend(_ InputDeviceProperties) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) InputDeviceRemove(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) CdromAppend(_ CdromProperties) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) CdromInsert(_ CdromProperties, _ int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) CdromRemove(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) CdromChangeMedia(devname, media string) error {
	media = strings.TrimSpace(media)

	if len(media) == 0 {
		return fmt.Errorf("empty cdrom media")
	}

	cd := r.Cdroms.Get(devname)

	if cd == nil {
		return &NotConnectedError{"instance_qemu", devname}
	}

	if cd.Media == media {
		return &AlreadyConnectedError{"instance_qemu", media}
	}

	if be, err := NewCdromBackend(media); err == nil {
		cd.MediaBackend = be
	} else {
		return err
	}

	var success bool

	var oldMedia string = cd.Media
	var oldMediaLocal bool

	if _, ok := cd.MediaBackend.(*block.Device); ok {
		oldMediaLocal = true
	}

	defer func() {
		// Remove "old" media from a chroot on success
		if oldMediaLocal && success {
			os.Remove(filepath.Join(CHROOTDIR, r.name, oldMedia))
		}
	}()

	// New media backend
	if be, err := NewCdromBackend(media); err == nil {
		cd.MediaBackend = be
	} else {
		return err
	}

	cd.Media = media

	// Map the original block device to a chroot using mknod
	if _, ok := cd.MediaBackend.(*block.Device); ok {
		chrootDevPath := filepath.Join(CHROOTDIR, r.name, cd.Media)

		defer func() {
			if !success {
				os.Remove(chrootDevPath)
			}
		}()

		stat := syscall.Stat_t{}

		if err := syscall.Stat(cd.Media, &stat); err != nil {
			return err
		}

		os.MkdirAll(filepath.Dir(chrootDevPath), 0755)

		if err := syscall.Mknod(chrootDevPath, syscall.S_IFBLK|uint32(os.FileMode(01640)), int(stat.Rdev)); err != nil {
			if os.IsExist(err) {
				return fmt.Errorf("device is already in use: %s", cd.Media)
			}
			return err
		}
		if err := os.Chown(chrootDevPath, r.uid, 0); err != nil {
			return err
		}
	}

	// Changes in QEMU

	wait := func(fn func(context.Context, string, uint64) (*qmp.Event, error), after time.Time) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if _, err := fn(ctx, cd.QdevID(), uint64(after.Unix())); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("change medium timeout error: failed to complete within 60 seconds")
			}
			return err
		}

		return nil
	}

	// Open the tray
	ts := time.Now()

	if err := r.mon.Run(qmp.Command{Name: "blockdev-open-tray", Arguments: &qemu_types.StrID{ID: cd.QdevID()}}, nil); err != nil {
		return err
	}

	// Wait until the tray is opened
	if err := wait(r.mon.WaitDeviceTrayOpenedEvent, ts); err != nil {
		return err
	}

	// Replace the media and close the tray
	opts := struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
	}{
		ID:       cd.QdevID(),
		Filename: media,
	}

	ts = time.Now()

	if err := r.mon.Run(qmp.Command{Name: "blockdev-change-medium", Arguments: &opts}, nil); err != nil {
		return err
	}

	// Wait until the tray is closed
	if err := wait(r.mon.WaitDeviceTrayClosedEvent, ts); err != nil {
		return err
	}

	success = true

	return nil
}

func (r *InstanceQemu) CdromRemoveMedia(devname string) error {
	cd := r.Cdroms.Get(devname)

	if cd == nil {
		return &NotConnectedError{"instance_conf", devname}
	}

	if len(cd.Media) == 0 {
		return nil
	}

	// Remove the existing block device from a chroot
	if _, ok := cd.MediaBackend.(*block.Device); ok {
		chrootDevPath := filepath.Join(CHROOTDIR, r.name, cd.Media)
		if err := os.Remove(chrootDevPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	// Change in QEMU

	wait := func(fn func(context.Context, string, uint64) (*qmp.Event, error), after time.Time) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		_, err := fn(ctx, cd.QdevID(), uint64(after.Unix()))
		if err != nil {
			if err == context.DeadlineExceeded {
				return fmt.Errorf("remove medium timeout error: failed to complete within 60 seconds")
			}
			return err
		}

		return nil
	}

	// Open the tray
	ts := time.Now()

	if err := r.mon.Run(qmp.Command{Name: "blockdev-open-tray", Arguments: &qemu_types.StrID{ID: cd.QdevID()}}, nil); err != nil {
		return err
	}

	// ... wait until the tray is opened
	if err := wait(r.mon.WaitDeviceTrayOpenedEvent, ts); err != nil {
		return err
	}

	// Remove the media and close the tray
	ts = time.Now()

	if err := r.mon.Run(qmp.Command{Name: "blockdev-remove-medium", Arguments: &qemu_types.StrID{ID: cd.QdevID()}}, nil); err != nil {
		return err
	}

	// ... wait until the tray is closed
	if err := wait(r.mon.WaitDeviceTrayClosedEvent, ts); err != nil {
		return err
	}

	// Ok, removed
	cd.Media = ""
	cd.MediaBackend = nil

	return nil
}

func (r *InstanceQemu) DiskAppend(_ Disk) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) DiskInsert(_ DiskProperties, _ int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) DiskRemove(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) DiskSetReadIops(diskname string, iops int) error {
	if iops < 0 {
		return fmt.Errorf("invalid iops value: cannot be less than 0")
	}

	d := r.Disks.Get(diskname)

	if d == nil {
		return &NotConnectedError{"instance_qemu", diskname}
	}

	opts := qemu_types.BlockIOThrottle{
		Device: d.BaseName(),
		IopsRd: iops,
		IopsWr: d.IopsWr,
	}

	if err := r.mon.Run(qmp.Command{Name: "block_set_io_throttle", Arguments: &opts}, nil); err != nil {
		return err
	}

	d.IopsRd = iops

	return nil
}

func (r *InstanceQemu) DiskSetWriteIops(diskname string, iops int) error {
	if iops < 0 {
		return fmt.Errorf("invalid iops value: cannot be less than 0")
	}

	d := r.Disks.Get(diskname)

	if d == nil {
		return &NotConnectedError{"instance_qemu", diskname}
	}

	opts := qemu_types.BlockIOThrottle{
		Device: d.BaseName(),
		IopsRd: d.IopsRd,
		IopsWr: iops,
	}

	if err := r.mon.Run(qmp.Command{Name: "block_set_io_throttle", Arguments: &opts}, nil); err != nil {
		return err
	}

	d.IopsWr = iops

	return nil
}

func (r InstanceQemu) DiskResizeQemuBlockdev(dpath string) error {
	d := r.Disks.Get(dpath)

	if d == nil {
		return &NotConnectedError{"instance_qemu", dpath}
	}

	opts := qemu_types.BlockResizeQuery{
		Device: d.BaseName(),
		Size:   1,
	}

	return r.mon.Run(qmp.Command{Name: "block_resize", Arguments: &opts}, nil)
}

func (r *InstanceQemu) DiskRemoveQemuBitmap(diskname string) error {
	d := r.Disks.Get(diskname)

	if d == nil {
		return &NotConnectedError{"instance_qemu", diskname}
	}

	if !d.HasBitmap {
		return nil
	}

	opts := qemu_types.BlockDirtyBitmapOptions{
		Node: d.Backend.BaseName(),
		Name: "backup",
	}

	return r.mon.Run(qmp.Command{Name: "block-dirty-bitmap-remove", Arguments: &opts}, nil)
}

func (r *InstanceQemu) NetIfaceAppend(_ NetIfaceProperties) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) NetIfaceRemove(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) NetIfaceSetQueues(_ string, _ int) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) NetIfaceSetUpScript(_, _ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) NetIfaceSetDownScript(_, _ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) NetIfaceSetLinkUp(ifname string) error {
	n := r.NetIfaces.Get(ifname)

	if n == nil {
		return &NotConnectedError{"instance_qemu", ifname}
	}

	linkState := struct {
		Name    string `json:"name"`
		Carrier bool   `json:"up"`
	}{
		n.QdevID(),
		true,
	}

	return r.mon.Run(qmp.Command{Name: "set_link", Arguments: &linkState}, nil)
}

func (r *InstanceQemu) NetIfaceSetLinkDown(ifname string) error {
	n := r.NetIfaces.Get(ifname)

	if n == nil {
		return &NotConnectedError{"instance_qemu", ifname}
	}

	linkState := struct {
		Name    string `json:"name"`
		Carrier bool   `json:"up"`
	}{
		n.QdevID(),
		false,
	}

	return r.mon.Run(qmp.Command{Name: "set_link", Arguments: &linkState}, nil)
}

func (r *InstanceQemu) initVSockDevice() error {
	vsock := ChannelVSock{}

	cidQomQuery := qemu_types.QomQuery{Path: "vsock_device", Property: "guest-cid"}

	if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &cidQomQuery}, &vsock.ContextID); err != nil {
		if _, ok := err.(*qmp.DeviceNotFound); ok {
			return nil
		}
		return err
	}

	// An addr/slot on the PCI bus
	var pciAddr string

	addrQomQuery := qemu_types.QomQuery{Path: "vsock_device", Property: "legacy-addr"}

	if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &addrQomQuery}, &pciAddr); err == nil {
		vsock.QemuAddr = fmt.Sprintf("0x%s", strings.Split(pciAddr, ".")[0])
	}

	r.VSockDevice = &vsock

	return nil
}

func (r *InstanceQemu) VSockDeviceAppend(opts ChannelVSockProperties) error {
	if r.VSockDevice != nil {
		return &AlreadyConnectedError{"instance_conf", "vsock device"}
	}

	if err := opts.Validate(true); err != nil {
		return err
	}

	vsock := ChannelVSock{
		ChannelVSockProperties: opts,
	}

	devOpts := qemu_types.VSockDeviceOptions{
		Driver:   "vhost-vsock-pci",
		ID:       "vsock_device",
		GuestCID: vsock.ContextID,
	}

	if err := r.mon.Run(qmp.Command{Name: "device_add", Arguments: &devOpts}, nil); err != nil {
		return err
	}

	r.VSockDevice = &vsock

	return nil
}

func (r *InstanceQemu) VSockDeviceRemove() error {
	if r.VSockDevice == nil {
		return &NotConnectedError{"instance_conf", "vsock device"}
	}

	if err := r.mon.Run(qmp.Command{Name: "device_del", Arguments: &qemu_types.StrID{ID: "vsock_device"}}, nil); err != nil {
		return err
	}

	r.VSockDevice = nil

	return nil
}

func (r *InstanceQemu) CloudInitSetMedia(media string) error {
	if r.CloudInitDrive == nil {
		return &NotConnectedError{"instance_qemu", "cloud-init drive"}
	}

	newdrive, err := NewCloudInitDrive(media)
	if err != nil {
		return err
	}

	newdrive.CloudInitDriveProperties.Driver = r.CloudInitDrive.Driver().String()
	newdrive.driver = r.CloudInitDrive.Driver()

	if _, ok := newdrive.Backend.(*file.Device); ok {
		if filepath.Dir(newdrive.Media) != filepath.Join(CONFDIR, r.name) {
			return fmt.Errorf("must be placed in the machine home directory: %s/", filepath.Join(CONFDIR, r.name))
		}
	}

	if ok, err := newdrive.Backend.IsAvailable(); err == nil {
		if !ok {
			return fmt.Errorf("cloud-init media is not available: %s", media)
		}
	} else {
		return fmt.Errorf("failed to check cloud-init media: %w", err)
	}

	curdrive := r.CloudInitDrive

	var success bool

	inChroot := func(p string) string {
		return filepath.Join(CHROOTDIR, r.name, p)
	}

	defer func() {
		// Remove "old" media from a chroot on success
		if success && newdrive.Media != curdrive.Media {
			os.Remove(inChroot(curdrive.Backend.FullPath()))
		}
	}()

	// Update in a chroot
	if newdrive.IsLocal() {
		if err := os.MkdirAll(filepath.Dir(inChroot(newdrive.Backend.FullPath())), 0755); err != nil {
			return err
		}

		defer func() {
			if !success && newdrive.Media != curdrive.Media {
				os.Remove(inChroot(newdrive.Backend.FullPath()))
			}
		}()

		switch newdrive.Backend.(type) {
		case *block.Device:
			stat := syscall.Stat_t{}

			if err := syscall.Stat(newdrive.Backend.FullPath(), &stat); err != nil {
				return err
			}

			if err := syscall.Mknod(inChroot(newdrive.Backend.FullPath()), syscall.S_IFBLK|uint32(os.FileMode(01640)), int(stat.Rdev)); err != nil {
				if !os.IsExist(err) {
					return err
				}
			}
		case *file.Device:
			err := func() error {
				src, err := os.Open(newdrive.Backend.FullPath())
				if err != nil {
					return err
				}
				defer src.Close()

				tempDst, err := os.CreateTemp(filepath.Dir(inChroot(newdrive.Backend.FullPath())), ".cidata-*")
				if err != nil {
					return err
				}
				defer tempDst.Close()

				defer func() {
					os.Remove(tempDst.Name())
				}()

				if _, err := io.Copy(tempDst, src); err != nil {
					return err
				}

				return os.Rename(tempDst.Name(), inChroot(newdrive.Backend.FullPath()))
			}()

			if err != nil {
				return fmt.Errorf("failed to copy cloud-init drive into the chroot directory: %w", err)
			}
		default:
			return &backend.UnknownBackendError{Path: newdrive.Media}
		}

		if err := os.Chown(inChroot(newdrive.Backend.FullPath()), r.uid, 0); err != nil {
			return err
		}
	}

	// Changes in QEMU

	wait := func(fn func(context.Context, string, uint64) (*qmp.Event, error), after time.Time) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if _, err := fn(ctx, newdrive.QdevID(), uint64(after.Unix())); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("change medium timeout error: failed to complete within 60 seconds")
			}
			return err
		}

		return nil
	}

	changeMediumArgs := struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
	}{
		ID:       "cidata",
		Filename: newdrive.Backend.FullPath(),
	}

	switch curdrive.Driver() {
	case CloudInitDriverType_IDE_CD:
		var ts time.Time

		// Open the tray
		ts = time.Now()

		if err := r.mon.Run(qmp.Command{Name: "blockdev-open-tray", Arguments: &qemu_types.StrID{ID: "cidata"}}, nil); err != nil {
			return err
		}

		// Wait until the tray is opened
		if err := wait(r.mon.WaitDeviceTrayOpenedEvent, ts); err != nil {
			return err
		}

		// Replace the media and close the tray
		ts = time.Now()

		if err := r.mon.Run(qmp.Command{Name: "blockdev-change-medium", Arguments: &changeMediumArgs}, nil); err != nil {
			return err
		}

		// Wait until the tray is closed
		if err := wait(r.mon.WaitDeviceTrayClosedEvent, ts); err != nil {
			return err
		}
	case CloudInitDriverType_FLOPPY:
		if err := r.mon.Run(qmp.Command{Name: "blockdev-change-medium", Arguments: &changeMediumArgs}, nil); err != nil {
			return err
		}
	}

	success = true

	r.CloudInitDrive = newdrive

	return nil
}

func (r *InstanceQemu) CloudInitSetDriver(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) CloudInitRemoveConf() error {
	return ErrNotImplemented
}

func (r *InstanceQemu) initExtKernel() error {
	r.Kernel.Image = r.startupConf.KernelGetImage()
	r.Kernel.Initrd = r.startupConf.KernelGetInitrd()
	r.Kernel.Cmdline = r.startupConf.KernelGetCmdline()
	r.Kernel.Modiso = r.startupConf.KernelGetModiso()

	return nil
}

func (r *InstanceQemu) KernelSetImage(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) KernelSetCmdline(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) KernelSetInitrd(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) KernelSetModiso(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) KernelRemoveConf() error {
	return ErrNotImplemented
}

func (r *InstanceQemu) initHostDevicePool() error {
	if c, ok := r.startupConf.(*StartupConf); ok {
		for _, key := range c.HostDevices.Pool.Keys() {
			r.HostDevices.Append(c.HostDevices.Get(key))
		}

		return nil
	}

	return fmt.Errorf("cannot init HostPCI-devices")
}

func (r *InstanceQemu) HostDeviceAppend(_ HostDeviceProperties) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) HostDeviceRemove(_ string) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) HostDeviceSetMultifunctionOption(_ string, _ bool) error {
	return ErrNotImplemented
}

func (r *InstanceQemu) HostDeviceSetPrimaryGPUOption(_ string, _ bool) error {
	return ErrNotImplemented
}

func (r InstanceQemu) VNCSetPassword(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("empty password string")
	}

	opts := struct {
		Password string `json:"password"`
	}{
		Password: s,
	}

	if err := r.mon.Run(qmp.Command{Name: "change-vnc-password", Arguments: opts}, nil); err != nil {
		return err
	}

	return nil
}
