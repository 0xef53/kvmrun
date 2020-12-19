package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	cg "github.com/0xef53/kvmrun/pkg/cgroup"
	"github.com/0xef53/kvmrun/pkg/kvmrun"
	"github.com/0xef53/kvmrun/pkg/qemu"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (l *Launcher) Start() error {
	if err := os.MkdirAll(filepath.Join(kvmrun.CONFDIR, l.vmname, ".runtime"), 0755); err != nil {
		return err
	}

	var vmconf kvmrun.Instance

	if c, err := kvmrun.GetInstanceConf(l.vmname); err != nil {
		return err
	} else {
		vmconf = c
	}
	if c, err := kvmrun.GetIncomingConf(l.vmname); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot parse incoming_config: %s", err)
		}
	} else {
		vmconf = c
	}

	args := make([]string, 0, 32)
	args = append(args, qemu.BINARY, "-machine", "accel=kvm:tcg", "-name", l.vmname)

	// Memory
	args = append(args, "-m", fmt.Sprintf("%dM", vmconf.GetTotalMem()))

	// Machine type
	if vmconf.GetMachineType() != "" {
		args = append(args, "-M", vmconf.GetMachineType())
	}

	// Disable default devices
	args = append(args, "-nodefaults", "-no-user-config")

	// CPU model
	if vmconf.GetCPUModel() != "" {
		args = append(args, "-cpu", vmconf.GetCPUModel())
	}

	// CPUs
	if vmconf.GetTotalCPUs() > 1 {
		args = append(args, "-smp", fmt.Sprintf("cpus=%d,maxcpus=%d", vmconf.GetActualCPUs(), vmconf.GetTotalCPUs()))
	}

	// Memory ballooning
	args = append(args, "-device", "virtio-balloon-pci,id=balloon0,bus=pci.0,addr=0x03")

	// Common virtio serial pci
	args = append(args, "-device", "virtio-serial-pci,bus=pci.0,addr=0x4")

	// Virtual console
	args = append(args, "-chardev", fmt.Sprintf("socket,id=virtcon,path=%s.virtcon,server,nowait", filepath.Join(kvmrun.QMPMONDIR, l.vmname)))
	args = append(args, "-device", "virtconsole,chardev=virtcon,name=console.0")

	// Virtio channels
	channels := vmconf.GetChannels()
	if len(channels) > 0 {
		for _, ch := range channels {
			chardevOpts := fmt.Sprintf("socket,id=%s,path=%s.%s,server,nowait", ch.CharDevName(), filepath.Join(kvmrun.QMPMONDIR, l.vmname), ch.ID)
			qdevOpts := fmt.Sprintf("virtserialport,id=%s,chardev=%s,name=%s", ch.QdevID(), ch.CharDevName(), ch.Name)
			if len(ch.Addr) > 0 {
				if nr, err := strconv.ParseInt(ch.Addr, 0, 32); err == nil {
					qdevOpts = fmt.Sprintf("%s,nr=%d", qdevOpts, nr)
				} else {
					return fmt.Errorf("failed to parse channel address: %s", err)
				}
			}
			args = append(args, "-chardev", chardevOpts)
			args = append(args, "-device", qdevOpts)
		}
	} else {
		// For compatibility with earlier versions
		args = append(args, "-chardev", fmt.Sprintf("socket,id=qga0,path=%s.qga,server,nowait", filepath.Join(kvmrun.QMPMONDIR, l.vmname)))
		args = append(args, "-device", "virtio-serial-pci,id=virtio-serial-qga0,bus=pci.0,addr=0x5")
		args = append(args, "-device", "virtserialport,chardev=qga0,name=org.guest-agent.0")
	}

	// VGA
	args = append(args, "-vga", "cirrus")

	// Block devices
	args = append(args, "-iscsi", "initiator-name=iqn.2008-11.org.linux-kvm:kvmrun")
	scsiBuses := make(map[string]struct{})
	for _, disk := range vmconf.GetDisks() {
		if !kvmrun.DiskDrivers.Exists(disk.Driver) {
			return fmt.Errorf("unknown disk driver: disk = %s, driver = %s", disk.Path, disk.Driver)
		}
		if disk.Driver == "scsi-hd" {
			busName, busAddr, _ := kvmrun.ParseSCSIAddr(disk.Addr)
			if _, ok := scsiBuses[busName]; !ok {
				busOpts := fmt.Sprintf("virtio-scsi-pci,id=%s,bus=pci.0", busName)
				if len(busAddr) > 0 {
					busOpts = fmt.Sprintf("%s,addr=%s", busOpts, busAddr)
				}
				args = append(args, "-device", busOpts)
				scsiBuses[busName] = struct{}{}
			}
		}
		args = append(args, disk.QemuCommandArgs()...)
	}

	// External Kernel
	if vmconf.GetKernelImage() != "" {
		args = append(args, "-kernel", filepath.Join(kvmrun.KERNELSDIR, vmconf.GetKernelImage()))
		if vmconf.GetKernelInitrd() != "" {
			args = append(args, "-initrd", filepath.Join(kvmrun.KERNELSDIR, vmconf.GetKernelInitrd()))
		}
		kparams := []string{"root=/dev/vda"}
		if vmconf.GetKernelCmdline() != "" {
			kparams = append(kparams, strings.Replace(vmconf.GetKernelCmdline(), ";", " ", -1))
		}
		args = append(args, "-append", strings.Join(kparams, " "))
		if vmconf.GetKernelModiso() != "" {
			args = append(args, "-drive", fmt.Sprintf("file=%s,if=none,media=cdrom,id=modiso,format=raw,aio=native,cache=none", filepath.Join(kvmrun.MODULESDIR, vmconf.GetKernelModiso())))
			args = append(args, "-device", "virtio-blk-pci,drive=modiso,id=modiso,bus=pci.0,addr=0x1f")
		}
	}

	// Network devices
	network := vmconf.GetNetIfaces()
	if len(network) > 0 {
		for _, iface := range network {
			if !kvmrun.NetDrivers.Exists(iface.Driver) {
				return fmt.Errorf("unknown network interface driver: ifname = %s, driver = %s", iface.Ifname, iface.Driver)
			}
			args = append(args, iface.QemuCommandArgs()...)
		}
	} else {
		args = append(args, "-net", "none")
	}

	// VNC
	args = append(args, "-vnc", fmt.Sprintf("%s:%d,password,websocket=%d", getVncHost(), vmconf.Uid(), kvmrun.WEBSOCKSPORT+vmconf.Uid()))

	// QMP monitor
	args = append(args, "-qmp", fmt.Sprintf("unix:%s.qmp0,server,nowait", filepath.Join(kvmrun.QMPMONDIR, l.vmname)))
	args = append(args, "-qmp", fmt.Sprintf("unix:%s.qmp1,server,nowait", filepath.Join(kvmrun.QMPMONDIR, l.vmname)))

	// Other options
	if os.Getenv("USE_NOREBOOT") != "" {
		args = append(args, "-no-reboot")
	}

	args = append(args, "-runas", l.vmname, "-chroot", filepath.Join(kvmrun.CHROOTDIR, l.vmname))

	// For migration
	if vmconf.IsIncoming() {
		args = append(args, "-incoming", fmt.Sprintf("tcp:%s:%d", getIncomingHost(), kvmrun.INCOMINGPORT+vmconf.Uid()))
	}

	// Extra args from extra file
	if _, err := os.Stat("extra"); err == nil {
		b, err := ioutil.ReadFile("extra")
		if err != nil {
			return err
		}
		args = append(args, strings.Split(string(b), "\n")...)
	}

	// Just show a command line if debug
	if os.Getenv("DEBUG") != "" {
		fmt.Println(strings.Join(args, " "))
		return nil
	}

	// Prepare chroot environment
	switch err := prepareChroot(vmconf); {
	case err == nil:
	case IsNonFatalError(err):
		Error.Println("non fatal:", err)
	default:
		return err
	}

	// CPU cgroup init
	if err := enableCgroupCPU(vmconf); err != nil {
		return fmt.Errorf("cpu cgroup init: %s", err)
	}

	for _, fname := range []string{"incoming_config", ".runtime/migration_stat"} {
		if err := os.RemoveAll(fname); err != nil {
			return err
		}
	}

	if err := os.Setenv("VMNAME", l.vmname); err != nil {
		return err
	}

	if vmconf.GetTotalMem() != vmconf.GetActualMem() {
		Info.Printf("actual memory size will be set to %d MB\n", vmconf.GetActualMem())
	}

	// Delayed init
	req := rpccommon.QemuInitRequest{
		Name:      l.vmname,
		Pid:       os.Getpid(),
		MemActual: uint64(vmconf.GetActualMem()) << 20,
	}
	if err := l.client.Request("RPC.InitQemuInstance", &req, nil); err != nil {
		return fmt.Errorf("failed to call RPC: %s", err)
	}

	// Startup config
	switch vmconf.(type) {
	case *kvmrun.IncomingConf:
		if err := vmconf.(*kvmrun.IncomingConf).SaveStartupConfig(); err != nil {
			return err
		}
	case *kvmrun.InstanceConf:
		if err := vmconf.(*kvmrun.InstanceConf).SaveStartupConfig(); err != nil {
			return err
		}
	}

	// Run QEMU process
	Info.Printf("starting qemu-kvm process: pid = %d\n", os.Getpid())

	return syscall.Exec(qemu.BINARY, args, os.Environ())
}

func lookRomfile(romfile string) (string, error) {
	possibleDirs := []string{
		".",
		"/usr/share/qemu",
		"/usr/lib/ipxe/qemu",
		"/usr/share/seabios",
		"/usr/share/ipxe",
	}

	for _, d := range possibleDirs {
		switch _, err := os.Stat(filepath.Join(d, romfile)); {
		case err == nil:
			return d, nil
		case os.IsNotExist(err):
			continue
		default:
			return "", err
		}
	}

	return "", fmt.Errorf("failed to find romfile: %s", romfile)
}

func prepareChroot(vmconf kvmrun.Instance) error {
	vmChrootDir := filepath.Join(kvmrun.CHROOTDIR, vmconf.Name())

	if err := os.MkdirAll(filepath.Join(vmChrootDir, "dev/net"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(vmChrootDir, "run/net"), 0755); err != nil {
		return err
	}

	// Channels and other guest interfaces
	if err := os.MkdirAll(filepath.Join(vmChrootDir, kvmrun.QMPMONDIR), 0755); err != nil {
		return err
	}
	if err := os.Chown(filepath.Join(vmChrootDir, kvmrun.QMPMONDIR), vmconf.Uid(), 0); err != nil {
		return err
	}

	oldmask := syscall.Umask(0000)
	defer syscall.Umask(oldmask)

	for _, disk := range vmconf.GetDisks() {
		if !disk.IsLocal() {
			continue
		}
		if err := os.MkdirAll(filepath.Join(vmChrootDir, filepath.Dir(disk.Path)), 0755); err != nil {
			return err
		}
		stat := syscall.Stat_t{}
		if err := syscall.Stat(disk.Path, &stat); err != nil {
			return fmt.Errorf("stat %s: %s", disk.Path, err)
		}
		if err := syscall.Mknod(filepath.Join(vmChrootDir, disk.Path), syscall.S_IFBLK|uint32(os.FileMode(01600)), int(stat.Rdev)); err != nil {
			return fmt.Errorf("mknod %s: %s", disk.Path, err)
		}
		if err := os.Chown(filepath.Join(vmChrootDir, disk.Path), vmconf.Uid(), 0); err != nil {
			return err
		}
	}

	for _, device := range []string{"/dev/net/tun", "/dev/vhost-net"} {
		stat := syscall.Stat_t{}
		if err := syscall.Stat(device, &stat); err != nil {
			return fmt.Errorf("stat %s: %s", device, err)
		}
		if err := syscall.Mknod(filepath.Join(vmChrootDir, device[1:]), syscall.S_IFCHR|uint32(os.FileMode(01666)), int(stat.Rdev)); err != nil {
			return fmt.Errorf("mknod %s: %s", device, err)
		}
	}

	syscall.Umask(oldmask)

	for _, iface := range vmconf.GetNetIfaces() {
		jStr, err := json.Marshal(iface)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(filepath.Join(vmChrootDir, "run/net", iface.Ifname), jStr, 0644); err != nil {
			return err
		}
	}

	for _, romfile := range []string{"efi-virtio.rom"} {
		dir, err := lookRomfile(romfile)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(vmChrootDir, dir), 0755); err != nil {
			return err
		}
		src, err := os.Open(filepath.Join(dir, romfile))
		if err != nil {
			return err
		}
		defer src.Close()
		dst, err := os.Create(filepath.Join(vmChrootDir, dir, romfile))
		if err != nil {
			return err
		}
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			return err
		}
	}

	copyFileContent := func(fname string) error {
		if err := os.MkdirAll(filepath.Join(vmChrootDir, filepath.Dir(fname)), 0755); err != nil {
			return err
		}
		src, err := os.Open(fname)
		if err != nil {
			return err
		}
		defer src.Close()
		dst, err := os.Create(filepath.Join(vmChrootDir, fname))
		if err != nil {
			return err
		}
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			return err
		}
		return nil
	}

	err := func() error {
		iscsiLib := "/usr/lib/x86_64-linux-gnu/qemu/block-iscsi.so"

		if err := copyFileContent(iscsiLib); err != nil {
			return err
		}

		lddBinary, err := exec.LookPath("ldd")
		if err != nil {
			return err
		}

		out, err := exec.Command(lddBinary, iscsiLib).CombinedOutput()
		if err != nil {
			return err
		}

		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if !strings.Contains(line, " => ") {
				continue
			}
			parts := strings.Fields(line)

			if err := copyFileContent(parts[2]); err != nil {
				return err
			}
		}

		return nil
	}()
	if err != nil {
		return &NonFatalError{"unable to prepare iSCSI libs: " + err.Error()}
	}

	return nil
}

func enableCgroupCPU(vmconf kvmrun.Instance) error {
	if vmconf.GetCPUQuota() == 0 {
		return nil
	}

	cpuGroup, err := cg.NewCpuGroup(filepath.Join("kvmrun", vmconf.Name()), os.Getpid())
	if err != nil {
		return err
	}

	cgconf := cg.Config{}
	if err := cpuGroup.Get(&cgconf); err != nil {
		return err
	}

	// If CPU quota is disabled in Kernel
	if cgconf.CpuPeriod == 0 {
		return cg.ErrCfsNotEnabled
	}

	cgconf.CpuQuota = (cgconf.CpuPeriod * int64(vmconf.GetCPUQuota())) / 100
	if err := cpuGroup.Set(&cgconf); err != nil {
		return err
	}

	return nil
}

func getIncomingHost() string {
	if v, ok := os.LookupEnv("INCOMING_HOST"); ok {
		return v
	}
	return "0.0.0.0"
}

func getVncHost() string {
	if v, ok := os.LookupEnv("VNC_HOST"); ok {
		return v
	}
	return "127.0.0.2"
}
