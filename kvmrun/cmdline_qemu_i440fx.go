package kvmrun

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xef53/kvmrun/internal/pci"
)

type qemuCommandLine_i440fx struct {
	*qemuCommandLine
}

func (b *qemuCommandLine_i440fx) cdromArgs(dev *Cdrom) []string {
	backendOpts := []string{
		fmt.Sprintf("file=%s", dev.Media),
		fmt.Sprintf("id=%s", dev.Name),
		"format=raw",
		"if=none",
		"aio=native",
		"cache=none",
		"detect-zeroes=on",
	}

	if dev.ReadOnly {
		backendOpts = append(backendOpts, "readonly")
	}

	deviceOpts := []string{
		dev.Driver,
		fmt.Sprintf("drive=%s", dev.Name),
		fmt.Sprintf("id=%s", dev.QdevID()),
	}

	switch dev.Driver {
	case "scsi-cd":
		// SCSI devices have channel, scsi-id, and lun parameters
		deviceOpts = append(deviceOpts, "channel=0,scsi-id=1")
		bus, _, lun := ParseSCSIAddr(dev.Addr)
		deviceOpts = append(deviceOpts, fmt.Sprintf("bus=%s.0", bus))
		if lun != "" {
			deviceOpts = append(deviceOpts, fmt.Sprintf("lun=%s", lun))
		}
	}

	if dev.Bootindex > 0 {
		deviceOpts = append(deviceOpts, fmt.Sprintf("bootindex=%d", dev.Bootindex))
	}

	return []string{"-drive", strings.Join(backendOpts, ","), "-device", strings.Join(deviceOpts, ",")}
}

func (b *qemuCommandLine_i440fx) diskArgs(disk *Disk) []string {
	backendOpts := []string{
		fmt.Sprintf("file=%s", disk.Path),
		fmt.Sprintf("id=%s", disk.BaseName()),
		"format=raw",
		"if=none",
		"aio=native",
		"cache=none",
		"detect-zeroes=on",
		fmt.Sprintf("iops_rd=%d", disk.IopsRd),
		fmt.Sprintf("iops_wr=%d", disk.IopsWr),
	}

	deviceOpts := []string{
		disk.Driver,
		fmt.Sprintf("drive=%s", disk.BaseName()),
		fmt.Sprintf("id=%s", disk.QdevID()),
	}

	switch disk.Driver {
	case "virtio-blk-pci":
		// PCI devices have the addr parameter
		deviceOpts = append(deviceOpts, "bus=pci.0")
		if disk.Addr != "" {
			deviceOpts = append(deviceOpts, fmt.Sprintf("addr=%s", disk.Addr))
		}
	case "scsi-hd":
		// SCSI devices have channel, scsi-id, and lun parameters
		deviceOpts = append(deviceOpts, "channel=0,scsi-id=1")
		bus, _, lun := ParseSCSIAddr(disk.Addr)
		deviceOpts = append(deviceOpts, fmt.Sprintf("bus=%s.0", bus))
		if lun != "" {
			deviceOpts = append(deviceOpts, fmt.Sprintf("lun=%s", lun))
		}
	}

	if disk.Bootindex > 0 {
		deviceOpts = append(deviceOpts, fmt.Sprintf("bootindex=%d", disk.Bootindex))
	}

	return []string{"-drive", strings.Join(backendOpts, ","), "-device", strings.Join(deviceOpts, ",")}
}

func (b *qemuCommandLine_i440fx) ifaceArgs(iface *NetIface) []string {
	backendOpts := []string{
		"tap",
		fmt.Sprintf("ifname=%s", iface.Ifname),
		fmt.Sprintf("id=%s", iface.Ifname),
		"vhost=on",
		fmt.Sprintf("script=%s", VMNETINIT),
		"downscript=no",
	}

	deviceOpts := []string{
		iface.Driver,
		fmt.Sprintf("netdev=%s", iface.Ifname),
		fmt.Sprintf("id=%s", iface.QdevID()),
		fmt.Sprintf("mac=%s", iface.HwAddr),
	}

	if NetDrivers.HotPluggable(iface.Driver) {
		deviceOpts = append(deviceOpts, "bus=pci.0")
		if iface.Addr != "" {
			deviceOpts = append(deviceOpts, fmt.Sprintf("addr=%s", iface.Addr))
		}
	}

	if iface.Bootindex > 0 {
		deviceOpts = append(deviceOpts, fmt.Sprintf("bootindex=%d", iface.Bootindex))
	}

	// Enable multi-queue on virtio-net-pci interface
	if iface.Driver == "virtio-net-pci" && iface.Queues > 1 {
		// "iface.Queues" -- is the number of queue pairs.
		backendOpts = append(backendOpts, fmt.Sprintf("queues=%d", 2*iface.Queues))
		// "iface.Queues" count vectors for TX (transmit) queues, the same for RX (receive) queues,
		// one for configuration purposes, and one for possible VQ (vector quantization) control.
		deviceOpts = append(deviceOpts, fmt.Sprintf("mq=on,vectors=%d", 2*iface.Queues+2))
	}

	return []string{"-netdev", strings.Join(backendOpts, ","), "-device", strings.Join(deviceOpts, ",")}
}

func (b *qemuCommandLine_i440fx) hostpciArgs(num int, dev *HostPCI, backend *pci.Device) []string {
	opts := func(hexaddr string, fn uint8) []string {
		v := []string{
			"vfio-pci",
			fmt.Sprintf("host=%s", hexaddr),
			"bus=pci.1",
		}
		if dev.Multifunction {
			v = append(v, fmt.Sprintf("id=hostpci%d.%d", num, fn))
			v = append(v, fmt.Sprintf("addr=0x%x.0x%x", num, fn))
		} else {
			v = append(v, fmt.Sprintf("id=hostpci%d", num))
			v = append(v, fmt.Sprintf("addr=0x%x", num))
		}
		if fn == 0 {
			if dev.PrimaryGPU {
				v = append(v, "x-vga=on")
			}
			if dev.Multifunction {
				v = append(v, "multifunction=on")
			}
		}
		return v
	}

	subdevices := backend.Subdevices()

	args := make([]string, 0, 2*(len(subdevices)+1))

	args = append(args, "-device", strings.Join(opts(backend.String(), 0), ","))

	if dev.Multifunction {
		for _, sub := range subdevices {
			args = append(args, "-device", strings.Join(opts(sub.String(), sub.AddrFunction()), ","))
		}
	}

	return args
}

func (b *qemuCommandLine_i440fx) gen() ([]string, error) {
	args := make([]string, 0, 96)

	args = append(args, QEMU_BINARY, "-machine", "accel=kvm:tcg", "-name", b.vmconf.Name())

	// Machine type
	if t := b.vmconf.GetMachineType(); len(t.String()) > 0 {
		args = append(args, "-M", t.String())
	}

	// Disable default devices
	args = append(args, "-nodefaults", "-no-user-config")

	// Firmware
	if fwimage := b.vmconf.GetFirmwareImage(); len(fwimage) > 0 {
		args = append(args, "-drive", fmt.Sprintf("if=pflash,unit=0,id=fwloader,format=raw,readonly=on,file=%s", fwimage))
		if fwflash := b.vmconf.GetFirmwareFlash(); fwflash != nil {
			args = append(args, "-drive", fmt.Sprintf("if=pflash,unit=1,id=fwflash,format=raw,file=%s", fwflash.Path))
		}
	}

	// Memory
	args = append(args, "-m", fmt.Sprintf("%dM", b.vmconf.GetTotalMem()))

	// CPU model
	if model := b.vmconf.GetCPUModel(); len(model) > 0 {
		args = append(args, "-cpu", model)
	}

	// CPUs
	if total := b.vmconf.GetTotalCPUs(); total > 1 {
		if sockets := b.vmconf.GetCPUSockets(); sockets > 0 {
			if total%sockets != 0 {
				return nil, fmt.Errorf("total CPU count must be multiple of socket count: %d %% %d != 0", total, sockets)
			}
			args = append(args, "-smp", fmt.Sprintf("cpus=%d,sockets=%d,cores=%d,maxcpus=%d", b.vmconf.GetActualCPUs(), sockets, total/sockets, total))
		} else {
			args = append(args, "-smp", fmt.Sprintf("cpus=%d,maxcpus=%d", b.vmconf.GetActualCPUs(), total))
		}
	}

	// Memory ballooning
	args = append(args, "-device", "virtio-balloon-pci,id=balloon0,bus=pci.0,addr=0x3")

	// Common virtio serial pci
	args = append(args, "-device", "virtio-serial-pci,bus=pci.0,addr=0x4")

	// Virtual console
	args = append(args, "-chardev", fmt.Sprintf("socket,id=virtcon,path=%s.virtcon,server,nowait", filepath.Join(QMPMONDIR, b.vmconf.Name())))
	args = append(args, "-device", "virtconsole,chardev=virtcon,name=console.0")

	var hasPrimaryGPU bool

	// PCI passthrough
	if devs := b.vmconf.GetHostPCIDevices(); len(devs) > 0 {
		// Dedicated PCI bus
		args = append(args, "-device", "pci-bridge,id=pci.1,chassis_nr=1,bus=pci.0,addr=0x7")
		for num, dev := range devs {
			backend, err := pci.LookupDevice(dev.Addr)
			if err != nil {
				return nil, err
			}

			if dev.Multifunction && !backend.HasMultifunctionFeature() {
				return nil, fmt.Errorf("multifunction is not supported: %s", backend.String())
			}

			args = append(args, b.hostpciArgs(num, &dev, backend)...)

			if dev.PrimaryGPU {
				hasPrimaryGPU = true
			}
		}
	}

	// VGA
	if hasPrimaryGPU {
		args = append(args, "-vga", "none", "-nographic")
	} else {
		args = append(args, "-vga", "cirrus")
	}

	// Input devices
	for _, dev := range b.vmconf.GetInputDevices() {
		switch dev.Type {
		case "usb-tablet":
			args = append(args, "-device", "piix3-usb-uhci,id=uhci,bus=pci.0,addr=0x6")
			args = append(args, "-device", "usb-tablet,id=tablet,bus=uhci.0")
		}
	}

	// CloudInit drive
	if cidrive := b.vmconf.GetCloudInitDrive(); cidrive != nil {
		if len(cidrive.Driver) == 0 {
			// to ensure backward compatibility
			cidrive.Driver = "floppy"
		}
		if !CloudInitDrivers.Exists(cidrive.Driver) {
			return nil, fmt.Errorf("unknown cloud-init device driver: %s", cidrive.Driver)
		}

		args = append(args, "-smbios", "type=1,serial=ds=nocloud")
		args = append(args, "-drive", fmt.Sprintf("file=%s,id=cidata,format=raw,media=cdrom,if=none,aio=native,cache=none,readonly", cidrive.Media))
		switch cidrive.Driver {
		case "floppy":
			args = append(args, "-device", "floppy,drive=cidata,id=cidata")
		case "ide-cd":
			args = append(args, "-device", "ide-cd,bus=ide.0,unit=1,drive=cidata,id=cidata")
		case "virtio-blk-pci":
			args = append(args, "-device", "virtio-blk-pci,drive=cidata,id=cidata,bus=pci.0,addr=0x1e")
		}
	}

	// Channels: default virtio serial port
	args = append(args, "-chardev", fmt.Sprintf("socket,id=qga0,path=%s.qga,server,nowait", filepath.Join(QMPMONDIR, b.vmconf.Name())))
	args = append(args, "-device", "virtio-serial-pci,id=virtio-serial-qga0,bus=pci.0,addr=0x5")
	args = append(args, "-device", "virtserialport,chardev=qga0,name=org.guest-agent.0")

	// Channels: virtio vsock device
	if vsockDev := b.vmconf.GetVSockDevice(); vsockDev != nil {
		var cid uint32
		if vsockDev.Auto {
			cid = uint32(os.Getpid())
		} else {
			cid = vsockDev.ContextID
		}
		if cid < 3 {
			return nil, ErrIncorrectContextID
		}
		qdevOpts := fmt.Sprintf("vhost-vsock-pci,id=vsock_device,guest-cid=%d", cid)
		if len(vsockDev.Addr) != 0 {
			qdevOpts = fmt.Sprintf("%s,addr=%s", qdevOpts, vsockDev.Addr)
		}
		args = append(args, "-device", qdevOpts)
	}

	// iSCSI parameters
	args = append(args, "-iscsi", "initiator-name=iqn.2008-11.org.linux-kvm:kvmrun")

	// Common SCSI bus
	scsiBuses := make(map[string]struct{})

	// Cdrom devices
	for _, dev := range b.vmconf.GetCdroms() {
		if !CdromDrivers.Exists(dev.Driver) {
			return nil, fmt.Errorf("unknown cdrom driver: device = %s, driver = %s", dev.Name, dev.Driver)
		}
		if dev.Driver == "scsi-cd" {
			busName, busAddr, _ := ParseSCSIAddr(dev.Addr)
			if _, ok := scsiBuses[busName]; !ok {
				busOpts := fmt.Sprintf("virtio-scsi-pci,id=%s,bus=pci.0", busName)
				if len(busAddr) > 0 {
					busOpts = fmt.Sprintf("%s,addr=%s", busOpts, busAddr)
				}
				args = append(args, "-device", busOpts)
				scsiBuses[busName] = struct{}{}
			}
		}
		args = append(args, b.cdromArgs(&dev)...)
	}

	for _, disk := range b.vmconf.GetDisks() {
		if !DiskDrivers.Exists(disk.Driver) {
			return nil, fmt.Errorf("unknown disk driver: disk = %s, driver = %s", disk.Path, disk.Driver)
		}
		if disk.Driver == "scsi-hd" {
			busName, busAddr, _ := ParseSCSIAddr(disk.Addr)
			if _, ok := scsiBuses[busName]; !ok {
				busOpts := fmt.Sprintf("virtio-scsi-pci,id=%s,bus=pci.0", busName)
				if len(busAddr) > 0 {
					busOpts = fmt.Sprintf("%s,addr=%s", busOpts, busAddr)
				}
				args = append(args, "-device", busOpts)
				scsiBuses[busName] = struct{}{}
			}
		}
		args = append(args, b.diskArgs(&disk)...)
	}

	// External Kernel
	if kernImage := b.vmconf.GetKernelImage(); len(kernImage) > 0 {
		args = append(args, "-kernel", filepath.Join(KERNELSDIR, kernImage))
		if initrd := b.vmconf.GetKernelInitrd(); len(initrd) > 0 {
			args = append(args, "-initrd", filepath.Join(KERNELSDIR, initrd))
		}
		kparams := []string{"root=/dev/vda"}
		if cmdline := b.vmconf.GetKernelCmdline(); len(cmdline) > 0 {
			kparams = append(kparams, strings.Replace(cmdline, ";", " ", -1))
		}
		args = append(args, "-append", strings.Join(kparams, " "))
		if modiso := b.vmconf.GetKernelModiso(); len(modiso) > 0 {
			args = append(args, "-drive", fmt.Sprintf("file=%s,if=none,media=cdrom,id=modiso,format=raw,aio=native,cache=none", filepath.Join(MODULESDIR, modiso)))
			args = append(args, "-device", "virtio-blk-pci,drive=modiso,id=modiso,bus=pci.0,addr=0x1f")
		}
	}

	// Network devices
	if network := b.vmconf.GetNetIfaces(); len(network) > 0 {
		for _, iface := range network {
			if !NetDrivers.Exists(iface.Driver) {
				return nil, fmt.Errorf("unknown network interface driver: ifname = %s, driver = %s", iface.Ifname, iface.Driver)
			}
			args = append(args, b.ifaceArgs(&iface)...)
		}
	} else {
		args = append(args, "-net", "none")
	}

	// VNC
	args = append(args, "-vnc", fmt.Sprintf("%s:%d,password,websocket=%d", b.VNCHost(), b.vmconf.Uid(), FIRST_WS_PORT+b.vmconf.Uid()))

	// QMP monitor
	args = append(args, "-qmp", fmt.Sprintf("unix:%s.qmp0,server,nowait", filepath.Join(QMPMONDIR, b.vmconf.Name())))
	args = append(args, "-qmp", fmt.Sprintf("unix:%s.qmp1,server,nowait", filepath.Join(QMPMONDIR, b.vmconf.Name())))

	// Other options
	if b.features.NoReboot {
		args = append(args, "-no-reboot")
	}

	// Run as a non-privileged user
	args = append(args, "-runas", b.vmconf.Name())

	// For migration
	if b.vmconf.IsIncoming() {
		args = append(args, "-incoming", fmt.Sprintf("tcp:%s:%d", b.IncomingHost(), FIRST_INCOMING_PORT+b.vmconf.Uid()))
	}

	// Extra args from extra file
	if _, err := os.Stat("extra"); err == nil {
		b, err := os.ReadFile("extra")
		if err != nil {
			return nil, err
		}
		args = append(args, strings.Split(string(b), "\n")...)
	}

	return args, nil
}
