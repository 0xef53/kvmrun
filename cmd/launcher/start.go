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

	pb "github.com/0xef53/kvmrun/api/services/system/v1"
	"github.com/0xef53/kvmrun/internal/qemu"
	"github.com/0xef53/kvmrun/kvmrun"

	cg "github.com/0xef53/go-cgroups"
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

	// Start proxy servers for disk backends
	if len(vmconf.GetProxyServers()) > 0 {
		if _, err := l.client.StartDiskBackendProxy(l.ctx, &pb.DiskBackendProxyRequest{Name: l.vmname}); err != nil {
			return err
		}
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
	Info.Printf("starting qemu-kvm process: pid = %d\n", os.Getpid())

	var qemuBinary string

	if v, ok := os.LookupEnv("QEMU_BINARY"); ok {
		qemuBinary = v
	} else {
		qemuBinary = qemu.BINARY
	}

	Info.Printf("qemu binary: %s\n", qemuBinary)

	return syscall.Exec(qemuBinary, args, os.Environ())
}

func lookForRomfile(romfile string) (string, error) {
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
	if err := os.MkdirAll(filepath.Join(vmChrootDir, ".tasks"), 0755); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(vmChrootDir, "pid"), []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
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
			if err := ioutil.WriteFile(filepath.Join(vmChrootDir, "run/backend_proxy"), b, 0644); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	for _, device := range []string{"/dev/net/tun", "/dev/vhost-net", "/dev/vhost-vsock"} {
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
		dir, err := lookForRomfile(romfile)
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

	relpath := filepath.Join(kvmrun.CGROOTPATH, vmconf.Name())

	cpuGroup, err := cg.NewCpuGroup(relpath, os.Getpid())
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
