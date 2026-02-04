package kvmrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	pciDevs []qemu_types.PCIInfo   `json:"-"`
	blkDevs []qemu_types.BlockInfo `json:"-"`
}

func (r *InstanceQemu_i440fx) init() error {
	var gr errgroup.Group

	r.pciDevs = make([]qemu_types.PCIInfo, 0, 3)

	if err := r.mon.Run(qmp.Command{Name: "query-pci", Arguments: nil}, &r.pciDevs); err != nil {
		return err
	}

	r.blkDevs = make([]qemu_types.BlockInfo, 0, 8)

	if err := r.mon.Run(qmp.Command{Name: "query-block", Arguments: nil}, &r.blkDevs); err != nil {
		return err
	}

	gr.Go(func() error { return r.initCdromPool() })
	gr.Go(func() error { return r.initDiskPool() })
	gr.Go(func() error { return r.initNetIfacePool() })
	gr.Go(func() error { return r.initCloudInitDrive() })

	return gr.Wait()
}

func (r *InstanceQemu_i440fx) initCdromPool() error {
	for _, dev := range r.blkDevs {
		if !strings.HasPrefix(dev.QdevPath, "cdrom_") {
			// skip non-cdrom devices
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

		qomQuery := qemu_types.QomQuery{Path: cdrom.QdevID(), Property: "type"}

		if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &qomQuery}, &cdrom.CdromProperties.Driver); err != nil {
			if _, ok := err.(*qmp.DeviceNotFound); ok {
				// not a cdrom device, skip and continue.
				continue
			}
			return err
		}

		cdrom.driver = CdromDriverTypeValue(cdrom.CdromProperties.Driver)

		if cdrom.Driver() == DriverType_UNKNOWN {
			// skip device with unknown driver type
			continue
		}

		switch cdrom.Driver() {
		case CdromDriverType_IDE_CD:
			// An addr/slot on the PCI bus
			var pciAddr string

			qomQuery := qemu_types.QomQuery{Path: cdrom.QdevID(), Property: "legacy-addr"}

			if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &qomQuery}, &pciAddr); err == nil {
				cdrom.QemuAddr = fmt.Sprintf("0x%s", strings.Split(pciAddr, ".")[0])
			}
		case CdromDriverType_SCSI_CD:
			// SCSI bus name/addr and lun of disk
			var parentBus string

			busQomQuery := qemu_types.QomQuery{Path: cdrom.QdevID(), Property: "parent_bus"}

			if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &busQomQuery}, &parentBus); err != nil {
				return err
			}
			// in:  /machine/peripheral/scsi0/virtio-backend/scsi0.0
			// out: scsi0
			parentBusName := strings.Split(filepath.Base(parentBus), ".")[0]

			var lun int

			lunQomQuery := qemu_types.QomQuery{Path: cdrom.QdevID(), Property: "lun"}

			if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &lunQomQuery}, &lun); err != nil {
				return err
			}

			cdrom.QemuAddr = fmt.Sprintf("%s:%s/%d", parentBusName, r.scsiBuses[parentBusName].Addr, lun)
		}

		cdrom.Readonly = dev.Inserted.ReadOnly

		r.Cdroms.Append(cdrom)
	}

	return nil
}

func (r *InstanceQemu_i440fx) CdromAppend(opts CdromProperties) error {
	if err := opts.Validate(true); err != nil {
		return err
	}

	driver := CdromDriverTypeValue(opts.Driver)

	if !driver.HotPluggable() {
		return fmt.Errorf("cdrom driver is not hot-pluggable: %s", opts.Driver)
	}

	if r.Cdroms.Exists(opts.Name) {
		return &AlreadyConnectedError{"instance_qemu", opts.Name}
	}

	// If set then CdromChangeMedia will be called at the end
	requestedMedia := opts.Media

	opts.Media = ""

	// Changes in QEMU

	cd := Cdrom{
		CdromProperties: opts,
		driver:          driver,
	}

	deviceOpts := qemu_types.CdromDeviceOptions{
		Driver: cd.Driver().String(),
		ID:     cd.QdevID(),
		Drive:  cd.Name,
	}

	switch cd.Driver() {
	case CdromDriverType_SCSI_CD:
		busName, _, _ := ParseSCSIAddr(cd.QemuAddr)

		if _, ok := r.scsiBuses[busName]; !ok {
			busOpts := qemu_types.SCSIHostBusDeviceOptions{
				Driver: "virtio-scsi-pci",
				ID:     busName,
			}

			if err := r.mon.Run(qmp.Command{Name: "device_add", Arguments: &busOpts}, nil); err != nil {
				return fmt.Errorf("device_add failed: %s", err)
			}
		}

		deviceOpts.Bus = fmt.Sprintf("%s.0", busName)
		deviceOpts.SCSI_ID = 1
	}

	hcmd := fmt.Sprintf("id=%s,if=none,aio=threads,detect-zeroes=on", cd.Name)

	if cd.Readonly {
		hcmd += ",readonly=on"
	}

	if _, err := r.mon.RunHuman(fmt.Sprintf("drive_add auto \"%s\"", hcmd)); err != nil {
		return fmt.Errorf("drive_add failed: %s", err)
	}

	if err := r.mon.Run(qmp.Command{Name: "device_add", Arguments: &deviceOpts}, nil); err != nil {
		return fmt.Errorf("device_add failed: %s", err)
	}

	r.Cdroms.Append(&cd)

	// Call CdromChangeMedia() if needed
	if len(requestedMedia) > 0 {
		return r.CdromChangeMedia(cd.Name, requestedMedia)
	}

	return nil
}

func (r *InstanceQemu_i440fx) CdromRemove(devname string) error {
	cd := r.Cdroms.Get(devname)

	if cd == nil {
		return &NotConnectedError{"instance_qemu", devname}
	}

	if !cd.Driver().HotPluggable() {
		return fmt.Errorf("cdrom driver is not hot-pluggable: %s", cd.Driver())
	}

	// Remove the existing block device from a chroot
	if _, ok := cd.MediaBackend.(*block.Device); ok {
		chrootDevPath := filepath.Join(CHROOTDIR, r.name, cd.Media)

		if err := os.Remove(chrootDevPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	// Changes in QEMU

	// Remove QEMU device
	ts := time.Now()

	switch cd.Driver() {
	case CdromDriverType_SCSI_CD:
		if err := r.mon.Run(qmp.Command{Name: "device_del", Arguments: &qemu_types.StrID{ID: cd.QdevID()}}, nil); err != nil {
			return fmt.Errorf("device_del error: %s", err)
		}
	}

	// Wait until the operation is completed
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := r.mon.WaitDeviceDeletedEvent(ctx, cd.QdevID(), uint64(ts.Unix())); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("device_del timeout error: failed to complete within 60 seconds")
		}
		return err
	}

	// Find and remove block backend
	blkDevs := make([]qemu_types.BlockInfo, 0, 8)

	if err := r.mon.Run(qmp.Command{Name: "query-block", Arguments: nil}, &blkDevs); err != nil {
		return err
	}

	for _, dev := range blkDevs {
		if dev.Device == cd.Name {
			if _, err := r.mon.RunHuman("drive_del " + cd.Name); err != nil {
				return fmt.Errorf("drive_del error: %s", err)
			}
		}
	}

	return r.Cdroms.Remove(cd.Name)
}

func (r *InstanceQemu_i440fx) initDiskPool() error {
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

	for _, dev := range r.blkDevs {
		// skip reserved names and empty devices
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

		qomQuery := qemu_types.QomQuery{Path: disk.QdevID(), Property: "type"}

		if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &qomQuery}, &disk.DiskProperties.Driver); err != nil {
			if _, ok := err.(*qmp.DeviceNotFound); ok && strings.HasPrefix(dev.QdevPath, "cdrom_") {
				// Possible it is a cdrom device. If so, skip and continue.
				continue
			}
			return err
		}

		disk.driver = DiskDriverTypeValue(disk.DiskProperties.Driver)

		if disk.Driver() == DriverType_UNKNOWN {
			// skip device with unknown driver type
			continue
		}

		switch disk.Driver() {
		case DiskDriverType_VIRTIO_BLK_PCI, DiskDriverType_IDE_HD:
			// An addr/slot on the PCI bus
			var pciAddr string

			qomQuery := qemu_types.QomQuery{Path: disk.QdevID(), Property: "legacy-addr"}

			if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &qomQuery}, &pciAddr); err == nil {
				disk.QemuAddr = fmt.Sprintf("0x%s", strings.Split(pciAddr, ".")[0])
			}
		case DiskDriverType_SCSI_HD:
			// SCSI bus name/addr and lun of disk
			var parentBus string

			busQomQuery := qemu_types.QomQuery{Path: disk.QdevID(), Property: "parent_bus"}

			if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &busQomQuery}, &parentBus); err != nil {
				return err
			}
			// in:  /machine/peripheral/scsi0/virtio-backend/scsi0.0
			// out: scsi0
			parentBusName := strings.Split(filepath.Base(parentBus), ".")[0]

			var lun int

			lunQomQuery := qemu_types.QomQuery{Path: disk.QdevID(), Property: "lun"}

			if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &lunQomQuery}, &lun); err != nil {
				return err
			}

			disk.QemuAddr = fmt.Sprintf("%s:%s/%d", parentBusName, r.scsiBuses[parentBusName].Addr, lun)
		}

		for _, m := range append(dev.DirtyBitmaps, dev.Inserted.DirtyBitmaps...) {
			if m.Name == "backup" {
				disk.HasBitmap = true
			}
		}

		if be, err := NewDiskBackend(disk.Path); err == nil {
			disk.Backend = be
		} else {
			return err
		}

		r.Disks.Append(disk)
	}

	return nil
}

func (r *InstanceQemu_i440fx) DiskAppend(opts DiskProperties) error {
	if err := opts.Validate(true); err != nil {
		return err
	}

	driver := DiskDriverTypeValue(opts.Driver)

	if !driver.HotPluggable() {
		return fmt.Errorf("disk driver is not hot-pluggable: %s", opts.Driver)
	}

	d := Disk{
		DiskProperties: opts,
		driver:         driver,
	}

	if be, err := NewDiskBackend(d.Path); err == nil {
		if ok, err := be.IsAvailable(); err == nil {
			if !ok {
				return fmt.Errorf("disk is not available: %s", opts.Path)
			}
		} else {
			return fmt.Errorf("failed to check disk: %w", err)
		}

		d.Backend = be
	} else {
		return err
	}

	if r.Disks.Exists(d.Backend.BaseName()) {
		return &AlreadyConnectedError{"instance_qemu", d.Path}
	}

	var success bool

	// Map the original block device to a chroot using mknod
	if _, ok := d.Backend.(*block.Device); ok {
		chrootDevPath := filepath.Join(CHROOTDIR, r.name, d.Path)

		defer func() {
			if !success {
				os.Remove(chrootDevPath)
			}
		}()

		stat := syscall.Stat_t{}

		if err := syscall.Stat(d.Path, &stat); err != nil {
			return err
		}

		os.MkdirAll(filepath.Dir(chrootDevPath), 0755)

		if err := syscall.Mknod(chrootDevPath, syscall.S_IFBLK|uint32(os.FileMode(01640)), int(stat.Rdev)); err != nil {
			if os.IsExist(err) {
				return fmt.Errorf("device is already in use: %s", d.Path)
			}
			return err
		}

		if err := os.Chown(chrootDevPath, r.uid, 0); err != nil {
			return err
		}
	}

	// Changes in QEMU

	// Add QEMU device
	devOpts := qemu_types.BlockDeviceOptions{
		Driver: d.Driver().String(),
		ID:     d.QdevID(),
		Drive:  d.BaseName(),
	}

	switch d.Driver() {
	case DiskDriverType_SCSI_HD:
		busName, _, _ := ParseSCSIAddr(d.QemuAddr)

		if _, ok := r.scsiBuses[busName]; !ok {
			busOpts := qemu_types.SCSIHostBusDeviceOptions{
				Driver: "virtio-scsi-pci",
				ID:     busName,
			}

			if err := r.mon.Run(qmp.Command{Name: "device_add", Arguments: &busOpts}, nil); err != nil {
				return fmt.Errorf("device_add failed: %s", err)
			}
		}

		devOpts.Bus = fmt.Sprintf("%s.0", busName)
		devOpts.SCSI_ID = 1
	}

	// Use HMP for add new block backend
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

	if err := r.mon.Run(qmp.Command{Name: "device_add", Arguments: &devOpts}, nil); err != nil {
		return fmt.Errorf("device_add failed: %s", err)
	}

	r.Disks.Append(&d)

	success = true

	return nil
}

func (r *InstanceQemu_i440fx) DiskRemove(diskname string) error {
	d := r.Disks.Get(diskname)

	if d == nil {
		return &NotConnectedError{"instance_qemu", diskname}
	}

	if !d.Driver().HotPluggable() {
		return fmt.Errorf("disk driver is not hot-pluggable: %s", d.Driver())
	}

	// Remove the existing block device from a chroot
	if _, ok := d.Backend.(*block.Device); ok {
		chrootDevPath := filepath.Join(CHROOTDIR, r.name, d.Path)

		if err := os.Remove(chrootDevPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	// Changes in QEMU

	// Remove QEMU device
	switch d.Driver() {
	case DiskDriverType_VIRTIO_BLK_PCI:
		ts := time.Now()

		if err := r.mon.Run(qmp.Command{Name: "device_del", Arguments: &qemu_types.StrID{ID: d.QdevID()}}, nil); err != nil {
			return fmt.Errorf("device_del error: %s", err)
		}

		// Wait until the operation is completed
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if _, err := r.mon.WaitDeviceDeletedEvent(ctx, d.QdevID(), uint64(ts.Unix())); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("device_del timeout error: failed to complete within 60 seconds")
			}
			return err
		}
	case DiskDriverType_SCSI_HD:
		if err := r.mon.Run(qmp.Command{Name: "device_del", Arguments: &qemu_types.StrID{ID: d.QdevID()}}, nil); err != nil {
			return fmt.Errorf("device_del error: %s", err)
		}
	}

	// Find and remove block backend
	blkDevs := make([]qemu_types.BlockInfo, 0, 8)

	if err := r.mon.Run(qmp.Command{Name: "query-block", Arguments: nil}, &blkDevs); err != nil {
		return err
	}

	for _, dev := range blkDevs {
		if dev.Device == d.BaseName() {
			if _, err := r.mon.RunHuman("drive_del " + d.BaseName()); err != nil {
				return fmt.Errorf("drive_del error: %s", err)
			}
		}
	}

	return r.Disks.Remove(d.Backend.BaseName())
}

func (r *InstanceQemu_i440fx) initNetIfacePool() error {
	for _, dev := range r.pciDevs[0].Devices {
		// {'class': 512, 'desc': 'Ethernet controller'}
		if dev.ClassInfo.Class != 512 {
			continue
		}

		netif := NetIface{QemuAddr: fmt.Sprintf("0x%x", dev.Slot)}

		typeQomQuery := qemu_types.QomQuery{Path: dev.QdevID, Property: "type"}

		if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &typeQomQuery}, &netif.NetIfaceProperties.Driver); err != nil {
			return err
		}

		netif.driver = NetDriverTypeValue(netif.NetIfaceProperties.Driver)

		if netif.Driver() == DriverType_UNKNOWN {
			// skip device with unknown driver type
			continue
		}

		macQomQuery := qemu_types.QomQuery{Path: dev.QdevID, Property: "mac"}

		if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &macQomQuery}, &netif.HwAddr); err != nil {
			return err
		}

		var mq bool
		var vectors uint32

		if netif.Driver() == NetDriverType_VIRTIO_NET_PCI {
			qomQuery := qemu_types.QomQuery{Path: dev.QdevID, Property: "mq"}

			if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &qomQuery}, &mq); err == nil {
				if mq {
					qomQuery := qemu_types.QomQuery{Path: dev.QdevID, Property: "vectors"}

					err = r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &qomQuery}, &vectors)
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

		if b, err := os.ReadFile(filepath.Join(CHROOTDIR, r.name, "run/net", netif.Ifname)); err == nil {
			if err := json.Unmarshal(b, &scripts); err != nil {
				return err
			}

			netif.Ifup = scripts.Ifup
			netif.Ifdown = scripts.Ifdown
		} else {
			return err
		}

		r.NetIfaces.Append(&netif)
	}

	return nil
}

func (r *InstanceQemu_i440fx) NetIfaceAppend(opts NetIfaceProperties) error {
	if err := opts.Validate(true); err != nil {
		return err
	}

	driver := NetDriverTypeValue(opts.Driver)

	if !driver.HotPluggable() {
		return fmt.Errorf("net interface driver is not hot-pluggable: %s", opts.Driver)
	}

	n := NetIface{
		NetIfaceProperties: opts,
		driver:             driver,
	}

	if r.NetIfaces.Exists(n.Ifname) {
		return &AlreadyConnectedError{"instance_qemu", n.Ifname}
	}

	// Changes in QEMU

	backendOpts := qemu_types.NetdevTapOptions{
		Type:       "tap",
		ID:         n.Ifname,
		Ifname:     n.Ifname,
		Vhost:      true,
		Script:     "no",
		Downscript: "no",
	}

	deviceOpts := qemu_types.NetDeviceOptions{
		Driver: n.Driver().String(),
		Netdev: n.Ifname,
		ID:     n.QdevID(),
		Mac:    n.HwAddr,
	}

	// Enable multi-queue on virtio-net-pci interface
	if n.Driver() == NetDriverType_VIRTIO_NET_PCI && n.Queues > 1 {
		// "iface.Queues" -- is the number of queue pairs.
		backendOpts.Queues = 2 * n.Queues

		// "n.Queues" count vectors for TX (transmit) queues, the same for RX (receive) queues,
		// one for configuration purposes, and one for possible VQ (vector quantization) control.
		deviceOpts.MQ = true
		deviceOpts.Vectors = 2*n.Queues + 2
	}

	// Add new tap-interface on the host side
	if err := AddTapInterface(n.Ifname, r.uid, deviceOpts.MQ); err != nil {
		return err
	}
	if err := SetInterfaceUp(n.Ifname); err != nil {
		return err
	}

	// Add netdev backend
	if err := r.mon.Run(qmp.Command{Name: "netdev_add", Arguments: &backendOpts}, nil); err != nil {
		return err
	}

	// Add QEMU device
	if err := r.mon.Run(qmp.Command{Name: "device_add", Arguments: &deviceOpts}, nil); err != nil {
		return err
	}

	// Save current configuration in a chroot
	if b, err := json.Marshal(n); err == nil {
		ifaceConf := filepath.Join(CHROOTDIR, r.name, "run/net", n.Ifname)

		if err := os.WriteFile(ifaceConf, b, 0644); err != nil {
			return err
		}
	} else {
		return err
	}

	r.NetIfaces.Append(&n)

	return nil
}

func (r *InstanceQemu_i440fx) NetIfaceRemove(ifname string) error {
	n := r.NetIfaces.Get(ifname)

	if n == nil {
		return &NotConnectedError{"instance_qemu", ifname}
	}

	if !n.Driver().HotPluggable() {
		return fmt.Errorf("net interface driver is not hot-pluggable: %s", n.Driver())
	}

	// Changes in QEMU

	// Remove QEMU device
	ts := time.Now()

	if err := r.mon.Run(qmp.Command{Name: "device_del", Arguments: &qemu_types.StrID{ID: n.QdevID()}}, nil); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := r.mon.WaitDeviceDeletedEvent(ctx, n.QdevID(), uint64(ts.Unix())); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("device_del timeout error: failed to complete within 60 seconds")
		}
		return err
	}

	// Remove netdev backend
	if err := r.mon.Run(qmp.Command{Name: "netdev_del", Arguments: &qemu_types.StrID{ID: n.Ifname}}, nil); err != nil {
		return err
	}

	if err := r.NetIfaces.Remove(n.Ifname); err != nil {
		return err
	}

	// Remove tap-interface on the host side
	if err := DelTapInterface(n.Ifname); err != nil {
		return fmt.Errorf("cannot remove the tap interface: %w", err)
	}

	ifaceConf := filepath.Join(CHROOTDIR, r.name, "run/net", n.Ifname)

	// Save current configuration in a chroot
	if err := os.Remove(ifaceConf); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (r *InstanceQemu_i440fx) initCloudInitDrive() error {
	for _, dev := range r.blkDevs {
		if dev.Device == "cidata" && dev.Inserted.File != "" {
			d, err := NewCloudInitDrive(dev.Inserted.File)
			if err != nil {
				return err
			}

			qomTypeQuery := qemu_types.QomQuery{
				Path:     "cidata",
				Property: "type",
			}

			if err := r.mon.Run(qmp.Command{Name: "qom-get", Arguments: &qomTypeQuery}, &d.CloudInitDriveProperties.Driver); err != nil {
				if _, ok := err.(*qmp.DeviceNotFound); ok {
					// unexpected, but not fatal.
					continue
				}
				return err
			}

			d.driver = CloudInitDriverTypeValue(d.CloudInitDriveProperties.Driver)

			r.CloudInitDrive = d
		}
	}

	return nil
}
