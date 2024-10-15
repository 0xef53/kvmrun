package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/system/v1"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/kvmrun/backend/file"
)

func (l *launcher) Cleanup() error {
	chrootDir := filepath.Join(kvmrun.CHROOTDIR, l.vmname)

	deconfigureIface := func(ifname string) error {
		iface := kvmrun.NetIface{}

		b, err := os.ReadFile(filepath.Join(chrootDir, "run/net", ifname))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &iface); err != nil {
			return err
		}

		// Running the external finish script
		if iface.Ifdown != "" {
			Info.Printf("starting the external finish script: %s, %s\n", iface.Ifname, iface.Ifdown)

			cmd := exec.Command(iface.Ifdown, iface.Ifname)

			cmd.Stdin = strings.NewReader(string(b))
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("external script failed: %s: %s\n", iface.Ifname, err)
			}
		}

		return kvmrun.DelTapInterface(iface.Ifname)
	}

	if files, err := os.ReadDir(filepath.Join(chrootDir, "run/net")); err == nil {
		for _, f := range files {
			if err := deconfigureIface(f.Name()); err == nil {
				Info.Println("cleanup: interface has been removed:", f.Name())
			} else {
				Error.Println("cleanup: failed to deconfigure interface:", err)
			}
		}
	} else {
		if !os.IsNotExist(err) {
			Error.Println("cleanup: failed to deconfigure network interfaces:", err)
		}
	}

	// Handle firmware flash image
	err := func() error {
		vmconf, err := kvmrun.GetStartupConf(l.vmname)
		if err != nil {
			return fmt.Errorf("unable to load startup config: %w", err)
		}

		if fwflash := vmconf.GetFirmwareFlash(); fwflash != nil {
			if inner, ok := fwflash.Backend.(*kvmrun.FirmwareBackend); ok {
				if _, ok := inner.DiskBackend.(*file.Device); ok {
					if b, err := file.New(filepath.Join(chrootDir, fwflash.Path)); err == nil {
						if size, err := b.Size(); size == 0 && err != nil {
							return os.ErrNotExist
						}
					} else {
						return err
					}

					// In case the machine turns off for the first time after migration to this host
					src, err := os.Open(filepath.Join(chrootDir, fwflash.Path))
					if err != nil {
						return err
					}
					defer src.Close()

					dst, err := os.Create(fwflash.Path)
					if err != nil {
						return err
					}
					defer dst.Close()

					if _, err := io.Copy(dst, src); err != nil {
						return err
					}

					return nil
				}
			}
		}

		return os.ErrNotExist
	}()

	if err == nil {
		Info.Println("cleanup: firmware flash image has been successfully copied to the confdir")
	} else {
		if !os.IsNotExist(err) {
			Error.Println("cleanup: failed to copy firmware flash image into the confdir:", err)
		}
	}

	if err := os.RemoveAll(chrootDir); err != nil {
		Error.Println("cleanup: failed to remove chroot environment:", err)
	}

	for _, ext := range []string{".qga", ".qmp0", ".qmp1", ".virtcon"} {
		os.Remove(filepath.Join(kvmrun.QMPMONDIR, l.vmname+ext))
	}

	if _, err := l.client.UnregisterQemuInstance(l.ctx, &pb.UnregisterQemuInstanceRequest{Name: l.vmname}); err != nil {
		Error.Println("cleanup: failed to release resources:", err)
	}

	if _, err := l.client.StopDiskBackendProxy(l.ctx, &pb.DiskBackendProxyRequest{Name: l.vmname}); err != nil {
		Error.Println("cleanup: failed to deconfigure disk backends proxy servers:", err)
	}

	return nil
}
