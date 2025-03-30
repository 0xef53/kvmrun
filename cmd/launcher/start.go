package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	pb "github.com/0xef53/kvmrun/api/services/system/v1"
	cg "github.com/0xef53/kvmrun/internal/cgroups"
	"github.com/0xef53/kvmrun/internal/fsutil"
	"github.com/0xef53/kvmrun/internal/helpers"
	"github.com/0xef53/kvmrun/internal/pci"
	"github.com/0xef53/kvmrun/internal/qemu"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/kvmrun/backend/block"
	"github.com/0xef53/kvmrun/kvmrun/backend/file"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (l *launcher) Start() error {
	if _, err := os.Stat(filepath.Join(kvmrun.CONFDIR, l.vmname, "down")); err == nil {
		Info.Println("machine will not start because it marked as disabled")
		os.Exit(0)
	} else {
		if !os.IsNotExist(err) {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Join(kvmrun.CONFDIR, l.vmname, ".runtime"), 0755); err != nil {
		return err
	}

	vmconf, err := func() (kvmrun.Instance, error) {
		c, err := kvmrun.GetIncomingConf(l.vmname)
		switch {
		case err == nil:
		case os.IsNotExist(err):
			return kvmrun.GetInstanceConf(l.vmname)
		default:
			return nil, fmt.Errorf("unable to parse incoming_config: %s", err)
		}
		return c, nil
	}()
	if err != nil {
		return err
	}

	// Build command line
	features := kvmrun.CommandLineFeatures{
		NoReboot: os.Getenv("USE_NOREBOOT") != "",
	}

	if v, ok := os.LookupEnv("INCOMING_HOST"); ok {
		features.IncomingHost = v
	}

	if v, ok := os.LookupEnv("VNC_HOST"); ok {
		features.VNCHost = v
	}

	args, err := kvmrun.GetCommandLine(vmconf, &features)
	if err != nil {
		Error.Fatalf("%s", err)
	}

	if t := vmconf.GetMachineType(); len(t.String()) > 0 {
		Info.Printf("requested machine type: %s\n", t)
	}

	// Just show a command line if debug
	if os.Getenv("DEBUG") != "" {
		fmt.Println(strings.Join(args, " "))
		return nil
	}

	var qemuRootDir string

	// AppConf with global Kvmrun options
	if resp, err := l.client.GetAppConf(l.ctx, new(empty.Empty)); err == nil {
		qemuRootDir = resp.AppConf.QemuRootdir
	} else {
		return fmt.Errorf("failed to request global Kvmrun configuration: %w", err)
	}

	if v, ok := os.LookupEnv("QEMU_ROOTDIR"); ok {
		qemuRootDir = v

		if _, err := os.Stat(qemuRootDir); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("QEMU root directory does not exist: %s", qemuRootDir)
			}
			return fmt.Errorf("failed to check QEMU root directory: %w", err)
		}
		Info.Printf("QEMU root directory: %s\n", qemuRootDir)
	} else {
		if err := os.Setenv("QEMU_ROOTDIR", qemuRootDir); err != nil {
			return err
		}
	}

	// Check all the PCI devices and detach them from the host
	if devs := vmconf.GetHostPCIDevices(); len(devs) > 0 {
		if err := loadVfioModule(); err != nil {
			return err
		}

		for _, hpci := range devs {
			pcidev, err := pci.LookupDevice(hpci.BackendAddr.String())
			if err != nil {
				return err
			}

			if pcidev.Enabled() {
				if pcidev.CurrentDriver() == "vfio-pci" {
					return fmt.Errorf("unable to work with open PCI device: %s", pcidev.String())
				}
			}

			// Switch to the "vfio-pci" driver this device and all its child devices
			// (all devices within the iommu_group are bound to their vfio bus driver)
			if err := pcidev.AssignDriver("vfio-pci"); err == nil {
				Info.Printf("PCI device has been detached from the host: %s\n", pcidev.String())
			} else {
				return fmt.Errorf("failed to detach PCI device %s: %w", pcidev.String(), err)
			}

			for _, sub := range pcidev.Subdevices() {
				if err := sub.AssignDriver("vfio-pci"); err == nil {
					Info.Printf("PCI device has been detached from the host: %s\n", sub.String())
				} else {
					return fmt.Errorf("failed to detach PCI device %s: %w", sub.String(), err)
				}
			}
		}
	}

	// Start proxy servers for disk backends
	if len(vmconf.GetProxyServers()) > 0 {
		if _, err := l.client.StartDiskBackendProxy(l.ctx, &pb.DiskBackendProxyRequest{Name: l.vmname}); err != nil {
			return err
		}
	}

	// Prepare chroot environment
	switch err := prepareChroot(vmconf, qemuRootDir); {
	case err == nil:
	case IsNonFatalError(err):
		Error.Println("non fatal:", err)
	default:
		return err
	}

	// CPU cgroup init
	if quota := vmconf.GetCPUQuota(); quota > 0 {
		mgr, err := cg.LoadManager(os.Getpid())
		if err != nil {
			return err
		}
		if err := mgr.SetCpuQuota(int64(quota)); err != nil {
			return err
		}
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
	req := pb.RegisterQemuInstanceRequest{
		Name:      l.vmname,
		PID:       int32(os.Getpid()),
		MemActual: int64(vmconf.GetActualMem()) << 20,
	}
	if _, err := l.client.RegisterQemuInstance(l.ctx, &req); err != nil {
		return err
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
	Info.Printf("starting QEMU process: pid = %d\n", os.Getpid())

	var qemuBinary string

	if v, ok := os.LookupEnv("QEMU_BINARY"); ok {
		qemuBinary = v
	} else {
		qemuBinary = qemu.BINARY
	}

	Info.Printf("QEMU binary: %s\n", qemuBinary)

	return syscall.Exec(qemuBinary, args, os.Environ())
}

// lookForRomfile returns a path to the romfile directory relative to the rootdir
// or an error if the romfile could not be found.
func lookForRomfile(romfile, rootdir string) (string, error) {
	possibleDirs := []string{
		"usr/share/qemu",
		"usr/lib/ipxe/qemu",
		"usr/share/seabios",
		"usr/share/ipxe",
	}

	// Check in current work directory
	if _, err := os.Stat(romfile); err == nil {
		return ".", nil
	}

	for _, d := range possibleDirs {
		switch _, err := os.Stat(filepath.Join(rootdir, d, romfile)); {
		case err == nil:
			return d, nil
		case os.IsNotExist(err):
			continue
		default:
			return "", err
		}
	}

	return "", fmt.Errorf("unable to find romfile: %s", romfile)
}

func prepareChroot(vmconf kvmrun.Instance, qemuRootDir string) error {
	vmChrootDir := filepath.Join(kvmrun.CHROOTDIR, vmconf.Name())

	if err := os.MkdirAll(filepath.Join(vmChrootDir, "dev/net"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(vmChrootDir, "run/net"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(vmChrootDir, ".tasks"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(vmChrootDir, "pid"), []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
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

	// A structure of proxies to use in Kvmrund
	if pp := vmconf.GetProxyServers(); len(pp) > 0 {
		if b, err := json.MarshalIndent(pp, "", "    "); err == nil {
			if err := os.WriteFile(filepath.Join(vmChrootDir, "run/backend_proxy"), b, 0644); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	for _, device := range []string{"/dev/net/tun", "/dev/vhost-net", "/dev/vhost-vsock", "/dev/null"} {
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
		if err := os.WriteFile(filepath.Join(vmChrootDir, "run/net", iface.Ifname), jStr, 0644); err != nil {
			return err
		}
	}

	// ROMs
	possibleDirs := []string{
		filepath.Join(qemuRootDir, "usr/share/qemu"),
		filepath.Join(qemuRootDir, "usr/lib/ipxe/qemu"),
		filepath.Join(qemuRootDir, "usr/share/seabios"),
		filepath.Join(qemuRootDir, "usr/share/ipxe"),
	}

	fmt.Printf("DEBUG(romfile) Start\n")
	for _, romname := range []string{"efi-virtio.rom"} {
		fmt.Printf("DEBUG(romfile) Check %s\n", romname)

		var rompath string

		// Check in current work directory
		if _, err := os.Stat(romname); err == nil {
			rompath = romname
			fmt.Printf("DEBUG(romfile) Found in the current work dir\n")
		} else {
			if _, p, err := helpers.LookForFile(romname, possibleDirs...); err == nil {
				rompath = p
				fmt.Printf("DEBUG(romfile) Found by LookForFile at %s\n", rompath)
			} else {
				return fmt.Errorf("unable to find romfile: %s", romname)
			}
		}

		var dstname string

		if p, err := filepath.Rel(qemuRootDir, rompath); err == nil {
			dstname = filepath.Join(vmChrootDir, p)
		} else {
			return err
		}

		if err := fsutil.Copy(rompath, dstname); err != nil {
			return err
		}
		fmt.Printf("DEBUG(romfile) Copy from %s to %s\n", rompath, dstname)
	}
	fmt.Printf("DEBUG(romfile) END\n")

	// Firmware flash image
	if fwflash := vmconf.GetFirmwareFlash(); fwflash != nil {
		if err := os.MkdirAll(filepath.Join(vmChrootDir, filepath.Dir(fwflash.Path)), 0755); err != nil {
			return err
		}
		if inner, ok := fwflash.Backend.(*kvmrun.FirmwareBackend); ok {
			switch inner.DiskBackend.(type) {
			case *block.Device:
				stat := syscall.Stat_t{}
				if err := syscall.Stat(fwflash.Path, &stat); err != nil {
					return fmt.Errorf("stat %s: %w", fwflash.Path, err)
				}
				if err := syscall.Mknod(filepath.Join(vmChrootDir, fwflash.Path), syscall.S_IFBLK|uint32(os.FileMode(01600)), int(stat.Rdev)); err != nil {
					return fmt.Errorf("mknod %s: %w", fwflash.Path, err)
				}
			case *file.Device:
				if _, ok := vmconf.(*kvmrun.IncomingConf); ok {
					// In case of incoming migration
					if err := fsutil.Copy(fwflash.Path, filepath.Join(vmChrootDir, fwflash.Path)); err != nil {
						return err
					}
					fmt.Printf("DEBUG(efivars) Copy from %s to %s\n", fwflash.Path, filepath.Join(vmChrootDir, fwflash.Path))
				} else {
					// It's a trick in case of outgoing migration? as QEMU checks for the presence of a "file" at this path
					if err := os.Symlink("/dev/null", filepath.Join(vmChrootDir, fwflash.Path)); err != nil {
						return err
					}
				}
			}
			if err := os.Chown(filepath.Join(vmChrootDir, fwflash.Path), vmconf.Uid(), 0); err != nil {
				return err
			}
		}
	}

	err := func() error {
		libfile := "/usr/lib/x86_64-linux-gnu/qemu/block-iscsi.so"

		if err := fsutil.Copy(libfile, filepath.Join(vmChrootDir, libfile)); err != nil {
			return err
		}
		fmt.Printf("DEBUG(iscsi) Copy from %s to %s\n", libfile, filepath.Join(vmChrootDir, libfile))

		lddBinary, err := exec.LookPath("ldd")
		if err != nil {
			return err
		}

		out, err := exec.Command(lddBinary, libfile).CombinedOutput()
		if err != nil {
			return err
		}

		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if !strings.Contains(line, " => ") {
				continue
			}
			parts := strings.Fields(line)

			if err := fsutil.Copy(parts[2], filepath.Join(vmChrootDir, parts[2])); err != nil {
				return err
			}
			fmt.Printf("DEBUG(iscsi) Copy from %s to %s\n", parts[2], filepath.Join(vmChrootDir, parts[2]))
		}

		return nil
	}()
	if err != nil {
		return &NonFatalError{"unable to prepare iSCSI libs: " + err.Error()}
	}

	return nil
}

func loadVfioModule() error {
	if _, err := os.Stat("/sys/bus/pci/drivers/vfio-pci"); err == nil {
		return nil
	}

	// Try to load anyway
	if _, err := exec.Command("modprobe", "vfio-pci").CombinedOutput(); err != nil {
		return fmt.Errorf("could not load vfio-pci module: modprobe failed with %s", err)
	}

	return nil
}
