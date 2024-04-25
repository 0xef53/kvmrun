package kvmrun

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	qemu_types "github.com/0xef53/kvmrun/internal/qemu/types"
	"github.com/0xef53/kvmrun/kvmrun/backend/block"

	qmp "github.com/0xef53/go-qmp/v2"
	"golang.org/x/sync/errgroup"
)

// InstanceQemu represents a configuration of a running QEMU instance.
type InstanceQemu_i440fx struct {
	*InstanceQemu

	pciDevs []qemu_types.PCIInfo `json:"-"`
}

func (r *InstanceQemu_i440fx) init() error {
	var gr errgroup.Group

	r.pciDevs = make([]qemu_types.PCIInfo, 0, 3)
	if err := r.mon.Run(qmp.Command{"query-pci", nil}, &r.pciDevs); err != nil {
		return err
	}

	gr.Go(func() error { return r.initCdroms() })
	gr.Go(func() error { return r.initStorage() })
	gr.Go(func() error { return r.initNetwork() })

	return gr.Wait()
}

func (r *InstanceQemu_i440fx) initCdroms() error {
	blkDevs := make([]qemu_types.BlockInfo, 0, 8)
	if err := r.mon.Run(qmp.Command{"query-block", nil}, &blkDevs); err != nil {
		return err
	}

	pool := make(CDPool, 0, len(blkDevs))
	for _, dev := range blkDevs {
		// Skip non-cdrom devices
		if !strings.HasPrefix(dev.QdevPath, "cdrom_") {
			continue
		}

		var devicePath string

		if strings.HasPrefix(dev.Inserted.File, "json:") {
			b := qemu_types.InsertedFileOptions{}
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
				return fmt.Errorf("unknown backing device driver: %s", b.File.Driver)
			}
		} else {
			devicePath = dev.Inserted.File
		}

		if dev.Inserted.BackingFileDepth > 0 {
			if len(dev.Inserted.BackingFile) != 0 {
				devicePath = dev.Inserted.BackingFile
			} else {
				devicePath = dev.Inserted.Image.Filename
			}
		}

		cdrom, err := NewCdrom(dev.Device, devicePath)
		if err != nil {
			return err
		}

		if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{cdrom.QdevID(), "type"}}, &cdrom.Driver); err != nil {
			if _, ok := err.(*qmp.DeviceNotFound); ok {
				// Not a cdrom device. Skip and continue.
				continue
			}
			return err
		}
		if !CdromDrivers.Exists(cdrom.Driver) {
			// Skip unknown driver
			continue
		}

		switch cdrom.Driver {
		case "ide-cd":
			// An addr/slot on the PCI bus
			var pciAddr string
			if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{cdrom.QdevID(), "legacy-addr"}}, &pciAddr); err == nil {
				cdrom.Addr = fmt.Sprintf("0x%s", strings.Split(pciAddr, ".")[0])
			}
		case "scsi-cd":
			// SCSI bus name/addr and lun of disk
			var parentBus string
			if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{cdrom.QdevID(), "parent_bus"}}, &parentBus); err != nil {
				return err
			}
			// in:  /machine/peripheral/scsi0/virtio-backend/scsi0.0
			// out: scsi0
			parentBusName := strings.Split(filepath.Base(parentBus), ".")[0]

			var lun int
			if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{cdrom.QdevID(), "lun"}}, &lun); err != nil {
				return err
			}

			cdrom.Addr = fmt.Sprintf("%s:%s/%d", parentBusName, r.scsiBuses[parentBusName].Addr, lun)
		}

		cdrom.ReadOnly = dev.Inserted.ReadOnly

		pool = append(pool, *cdrom)
	}

	r.Cdroms = pool

	return nil
}

func (r *InstanceQemu_i440fx) AppendCdrom(d Cdrom) error {
	if r.Cdroms.Exists(d.Name) {
		return &AlreadyConnectedError{"instance_qemu", d.Name}
	}

	if !CdromDrivers.Exists(d.Driver) {
		return fmt.Errorf("unknown device driver: %s", d.Driver)
	}

	if !CdromDrivers.HotPluggable(d.Driver) {
		return fmt.Errorf("driver is not hotpuggable: %s", d.Driver)
	}

	var success bool

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

	deviceOpts := qemu_types.DeviceOptions{
		Driver: d.Driver,
		Id:     d.QdevID(),
		Drive:  d.Name,
	}

	switch d.Driver {
	case "scsi-cd":
		busName, _, _ := ParseSCSIAddr(d.Addr)
		if _, ok := r.scsiBuses[busName]; !ok {
			busOpts := qemu_types.DeviceOptions{
				Driver: "virtio-scsi-pci",
				Id:     busName,
			}
			if err := r.mon.Run(qmp.Command{"device_add", &busOpts}, nil); err != nil {
				return fmt.Errorf("device_add failed: %s", err)
			}
		}
		deviceOpts.Bus = fmt.Sprintf("%s.0", busName)
		deviceOpts.SCSI_ID = 1
	}

	hcmd := fmt.Sprintf("file=%s,id=%s,format=raw,if=none,aio=native,cache=none,detect-zeroes=on", d.Media, d.Name)

	if d.ReadOnly {
		hcmd += ",readonly"
	}

	if _, err := r.mon.RunHuman(fmt.Sprintf("drive_add auto \"%s\"", hcmd)); err != nil {
		return fmt.Errorf("drive_add failed: %s", err)
	}

	if err := r.mon.Run(qmp.Command{"device_add", &deviceOpts}, nil); err != nil {
		return fmt.Errorf("device_add failed: %s", err)
	}

	r.Cdroms.Append(&d)

	success = true

	return nil
}

func (r *InstanceQemu_i440fx) RemoveCdrom(name string) error {
	d := r.Cdroms.Get(name)
	if d == nil {
		return &NotConnectedError{"instance_qemu", name}
	}

	if !CdromDrivers.HotPluggable(d.Driver) {
		return fmt.Errorf("driver is not hotpuggable: %s", d.Driver)
	}

	if _, ok := d.Backend.(*block.Device); ok {
		devpath := filepath.Join(CHROOTDIR, r.name, d.Media)
		if err := os.Remove(devpath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	ts := time.Now()

	switch d.Driver {
	case "scsi-cd":
		if err := r.mon.Run(qmp.Command{"device_del", &qemu_types.StrID{d.QdevID()}}, nil); err != nil {
			return fmt.Errorf("device_del error: %s", err)
		}
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

	blkDevs := make([]qemu_types.BlockInfo, 0, 8)
	if err := r.mon.Run(qmp.Command{"query-block", nil}, &blkDevs); err != nil {
		return err
	}

	for _, dev := range blkDevs {
		if dev.Device == d.Name {
			if _, err := r.mon.RunHuman("drive_del " + d.Name); err != nil {
				return fmt.Errorf("drive_del error: %s", err)
			}
		}
	}

	return r.Cdroms.Remove(d.Name)
}

func (r *InstanceQemu_i440fx) initStorage() error {
	r.scsiBuses = make(map[string]*SCSIBusInfo)
	for _, dev := range r.pciDevs[0].Devices {
		// desc:      SCSI controller
		// class:     256
		// id.device: 4100
		if !(dev.ClassInfo.Class == 256 && dev.ID.Device == 4100) {
			continue
		}
		r.scsiBuses[dev.QdevID] = &SCSIBusInfo{"virtio-scsi-pci", fmt.Sprintf("0x%x", dev.Slot)}
	}

	blkDevs := make([]qemu_types.BlockInfo, 0, 8)
	if err := r.mon.Run(qmp.Command{"query-block", nil}, &blkDevs); err != nil {
		return err
	}

	pool := make(DiskPool, 0, len(blkDevs))
	for _, dev := range blkDevs {
		// Skip reserved names and empty devices
		if dev.Device == "modiso" || dev.Device == "cidata" || dev.Device == "fwloader" || dev.Device == "fwflash" {
			continue
		}
		if dev.Inserted.File == "" {
			continue
		}

		var devicePath string

		if strings.HasPrefix(dev.Inserted.File, "json:") {
			b := qemu_types.InsertedFileOptions{}
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
				return fmt.Errorf("unknown backing device driver: %s", b.File.Driver)
			}
		} else {
			devicePath = dev.Inserted.File
		}

		if dev.Inserted.BackingFileDepth > 0 {
			if len(dev.Inserted.BackingFile) != 0 {
				devicePath = dev.Inserted.BackingFile
			} else {
				devicePath = dev.Inserted.Image.Filename
			}
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

		if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{disk.QdevID(), "type"}}, &disk.Driver); err != nil {
			if _, ok := err.(*qmp.DeviceNotFound); ok && strings.HasPrefix(dev.QdevPath, "cdrom_") {
				// Possible it is a cdrom device. If so, skip and continue.
				continue
			}
			return err
		}
		if !DiskDrivers.Exists(disk.Driver) {
			continue
		}

		switch disk.Driver {
		case "virtio-blk-pci", "ide-hd":
			// An addr/slot on the PCI bus
			var pciAddr string
			if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{disk.QdevID(), "legacy-addr"}}, &pciAddr); err == nil {
				disk.Addr = fmt.Sprintf("0x%s", strings.Split(pciAddr, ".")[0])
			}
		case "scsi-hd":
			// SCSI bus name/addr and lun of disk
			var parentBus string
			if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{disk.QdevID(), "parent_bus"}}, &parentBus); err != nil {
				return err
			}
			// in:  /machine/peripheral/scsi0/virtio-backend/scsi0.0
			// out: scsi0
			parentBusName := strings.Split(filepath.Base(parentBus), ".")[0]

			var lun int
			if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{disk.QdevID(), "lun"}}, &lun); err != nil {
				return err
			}

			disk.Addr = fmt.Sprintf("%s:%s/%d", parentBusName, r.scsiBuses[parentBusName].Addr, lun)
		}
		for _, m := range append(dev.DirtyBitmaps, dev.Inserted.DirtyBitmaps...) {
			if m.Name == "backup" {
				disk.HasBitmap = true
			}
		}

		pool = append(pool, *disk)
	}

	r.Disks = pool

	return nil
}

func (r *InstanceQemu_i440fx) AppendDisk(d Disk) error {
	if r.Disks.Exists(d.Path) {
		return &AlreadyConnectedError{"instance_qemu", d.Path}
	}

	if !DiskDrivers.HotPluggable(d.Driver) {
		return fmt.Errorf("unknown hotpuggable disk driver: %s", d.Driver)
	}

	var success bool

	if _, ok := d.Backend.(*block.Device); ok {
		devpath := filepath.Join(CHROOTDIR, r.name, d.Path)

		defer func() {
			if !success {
				os.Remove(devpath)
			}
		}()

		stat := syscall.Stat_t{}
		if err := syscall.Stat(d.Path, &stat); err != nil {
			return err
		}

		os.MkdirAll(filepath.Dir(devpath), 0755)

		if err := syscall.Mknod(devpath, syscall.S_IFBLK|uint32(os.FileMode(01640)), int(stat.Rdev)); err != nil {
			if os.IsExist(err) {
				return fmt.Errorf("device is already in use: %s", d.Path)
			}
			return err
		}
		if err := os.Chown(devpath, r.uid, 0); err != nil {
			return err
		}
	}

	devOpts := qemu_types.DeviceOptions{
		Driver: d.Driver,
		Id:     d.QdevID(),
		Drive:  d.BaseName(),
	}

	switch d.Driver {
	case "scsi-hd":
		busName, _, _ := ParseSCSIAddr(d.Addr)
		if _, ok := r.scsiBuses[busName]; !ok {
			busOpts := qemu_types.DeviceOptions{
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

	success = true

	return nil
}

func (r *InstanceQemu_i440fx) RemoveDisk(dpath string) error {
	d := r.Disks.Get(dpath)
	if d == nil {
		return &NotConnectedError{"instance_qemu", dpath}
	}

	if !DiskDrivers.HotPluggable(d.Driver) {
		return fmt.Errorf("unknown hotpuggable disk driver: %s", d.Driver)
	}

	if _, ok := d.Backend.(*block.Device); ok {
		devpath := filepath.Join(CHROOTDIR, r.name, d.Path)
		if err := os.Remove(devpath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	// Remove from the guest
	switch d.Driver {
	case "virtio-blk-pci":
		ts := time.Now()
		if err := r.mon.Run(qmp.Command{"device_del", &qemu_types.StrID{d.QdevID()}}, nil); err != nil {
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
		if err := r.mon.Run(qmp.Command{"device_del", &qemu_types.StrID{d.QdevID()}}, nil); err != nil {
			return fmt.Errorf("device_del error: %s", err)
		}
	}

	blkDevs := make([]qemu_types.BlockInfo, 0, 8)
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

	return r.Disks.Remove(d.Path)
}

func (r *InstanceQemu_i440fx) initNetwork() error {
	pool := make(NetifPool, 0, 8)
	for _, dev := range r.pciDevs[0].Devices {
		// {'class': 512, 'desc': 'Ethernet controller'}
		if dev.ClassInfo.Class != 512 {
			continue
		}

		netif := NetIface{Addr: fmt.Sprintf("0x%x", dev.Slot)}

		if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{dev.QdevID, "type"}}, &netif.Driver); err != nil {
			return err
		}
		if !NetDrivers.Exists(netif.Driver) {
			continue
		}

		if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{dev.QdevID, "mac"}}, &netif.HwAddr); err != nil {
			return err
		}

		var mq bool
		var vectors uint32

		if netif.Driver == "virtio-net-pci" {
			if err := r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{dev.QdevID, "mq"}}, &mq); err == nil {
				if mq {
					err = r.mon.Run(qmp.Command{"qom-get", &qemu_types.QomQuery{dev.QdevID, "vectors"}}, &vectors)
					if err != nil {
						return err
					}
					if vectors > 4 {
						netif.Queues = int(vectors-2) / 2
					}
				}
			} else {
				return err
			}
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

func (r *InstanceQemu_i440fx) AppendNetIface(iface NetIface) error {
	if len(iface.Ifname) == 0 {
		return fmt.Errorf("undefined network interface name")
	}

	if r.NetIfaces.Exists(iface.Ifname) {
		return &AlreadyConnectedError{"instance_qemu", iface.Ifname}
	}

	if !NetDrivers.HotPluggable(iface.Driver) {
		return fmt.Errorf("unknown hotpuggable network interface driver: %s", iface.Driver)
	}

	if _, err := net.ParseMAC(iface.HwAddr); err != nil {
		return err
	}

	hostOpts := qemu_types.NetdevTapOptions{
		Type:       "tap",
		ID:         iface.Ifname,
		Ifname:     iface.Ifname,
		Vhost:      true,
		Script:     "no",
		Downscript: "no",
	}
	opts := qemu_types.DeviceOptions{
		Driver: iface.Driver,
		Netdev: iface.Ifname,
		Id:     iface.QdevID(),
		Mac:    iface.HwAddr,
	}

	// Enable multi-queue on virtio-net-pci interface
	if iface.Driver == "virtio-net-pci" && iface.Queues > 1 {
		// "iface.Queues" -- is the number of queue pairs.
		hostOpts.Queues = 2 * iface.Queues

		// "iface.Queues" count vectors for TX (transmit) queues, the same for RX (receive) queues,
		// one for configuration purposes, and one for possible VQ (vector quantization) control.
		opts.MQ = true
		opts.Vectors = 2*iface.Queues + 2
	}

	if err := AddTapInterface(iface.Ifname, r.uid, opts.MQ); err != nil {
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

func (r *InstanceQemu_i440fx) RemoveNetIface(ifname string) error {
	iface := r.NetIfaces.Get(ifname)
	if iface == nil {
		return &NotConnectedError{"instance_qemu", ifname}
	}

	if !NetDrivers.HotPluggable(iface.Driver) {
		return fmt.Errorf("unknown hotpuggable network interface driver: %s", iface.Driver)
	}

	// Remove from the guest and wait until the operation is completed
	ts := time.Now()
	if err := r.mon.Run(qmp.Command{"device_del", &qemu_types.StrID{iface.QdevID()}}, nil); err != nil {
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
	if err := r.mon.Run(qmp.Command{"netdev_del", &qemu_types.StrID{ifname}}, nil); err != nil {
		return err
	}

	if err := r.NetIfaces.Remove(ifname); err != nil {
		return err
	}

	if err := DelTapInterface(ifname); err != nil {
		return fmt.Errorf("cannot remove the tap interface: %s", err)
	}

	ifaceConf := filepath.Join(CHROOTDIR, r.name, "run/net", ifname)
	if err := os.Remove(ifaceConf); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
