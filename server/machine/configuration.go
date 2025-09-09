package machine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/internal/fsutil"
	"github.com/0xef53/kvmrun/internal/osuser"
	"github.com/0xef53/kvmrun/internal/utils"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

func (s *Server) Create(ctx context.Context, vmname string, opts *kvmrun.InstanceProperties, qemuRootDir string) (*kvmrun.Machine, error) {
	if err := kvmrun.ValidateMachineName(vmname); err != nil {
		return nil, err
	}

	if opts == nil {
		return nil, fmt.Errorf("empty machine opts")
	} else {
		if err := opts.Validate(true); err != nil {
			return nil, err
		}
	}

	qemuRootDir = strings.TrimSpace(qemuRootDir)

	if len(qemuRootDir) == 0 {
		qemuRootDir = s.AppConf.Kvmrun.QemuRootDir
	}

	vmdir := filepath.Join(kvmrun.CONFDIR, vmname)

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		if _, err := os.Stat(filepath.Join(vmdir, "config")); err == nil {
			return fmt.Errorf("%w: %s", kvmrun.ErrAlreadyExists, vmname)
		} else {
			if !os.IsNotExist(err) {
				return err
			}
		}

		var success bool

		if err := os.MkdirAll(vmdir, 0755); err != nil {
			return err
		}
		defer func() {
			if !success {
				os.RemoveAll(vmdir)
			}
		}()

		if opts.Firmware != nil {
			if len(opts.Firmware.Flash) > 0 {
				if fi, err := os.Stat(opts.Firmware.Flash); err == nil {
					if fi.IsDir() {
						return fmt.Errorf("not a file: %s", opts.Firmware.Flash)
					}
				} else {
					if os.IsNotExist(err) {
						return err
					}
					opts.Firmware.Flash = ""
				}
			}

			switch opts.Firmware.Image {
			case "bios", "legacy":
				opts.Firmware.Image = ""
				opts.Firmware.Flash = ""
			case "efi", "uefi", "ovmf":
				possibleDirs := []string{
					filepath.Join(qemuRootDir, "usr/share/OVMF"),
					filepath.Join(qemuRootDir, "usr/share/ovmf"),
					filepath.Join(qemuRootDir, "usr/share/qemu"),
				}

				// Copy OVMF_CODE.fd to a virt.machine config dir
				_, fname, err := utils.LookForFile("OVMF_CODE.fd", possibleDirs...)
				if err != nil {
					return err
				}
				l.WithField("file", filepath.Base(fname)).Infof("Found at %s", fname)

				if err := fsutil.Copy(fname, filepath.Join(vmdir, "config_eficode")); err != nil {
					return fmt.Errorf("failed to copy config_eficode: %w", err)
				}
				l.WithField("file", filepath.Base(fname)).Infof("Copy to %s", filepath.Join(vmdir, "config_eficode"))

				opts.Firmware.Image = filepath.Join(vmdir, "config_eficode")

				// Copy OVMF_VARS.fd to a virt.machine config dir
				if len(opts.Firmware.Flash) == 0 {
					err := func() error {
						_, fname, err := utils.LookForFile("OVMF_VARS.fd", possibleDirs...)
						if err != nil {
							return err
						}
						l.WithField("file", filepath.Base(fname)).Infof("Found at %s", fname)

						if err := fsutil.Copy(fname, filepath.Join(vmdir, "config_efivars")); err != nil {
							return fmt.Errorf("failed to copy config_efivars: %w", err)
						}
						l.WithField("file", filepath.Base(fname)).Infof("Copy to %s", filepath.Join(vmdir, "config_efivars"))

						return nil
					}()

					if err != nil && !os.IsExist(err) {
						return err
					}

					opts.Firmware.Flash = filepath.Join(vmdir, "config_efivars")
				}
			}
		}

		if _, err := osuser.CreateUser(vmname); err != nil {
			return err
		}
		defer func() {
			if !success {
				osuser.RemoveUser(vmname)
			}
		}()

		vmc := kvmrun.NewInstanceConf(vmname)

		vmc.MemorySetTotal(opts.Memory.Total)
		vmc.MemorySetActual(opts.Memory.Actual)
		vmc.CPUSetTotal(opts.CPU.Total)
		vmc.CPUSetActual(opts.CPU.Actual)
		vmc.CPUSetQuota(opts.CPU.Quota)
		vmc.CPUSetModel(opts.CPU.Model)

		if opts.Firmware != nil {
			vmc.FirmwareSetImage(opts.Firmware.Image)
			vmc.FirmwareSetFlash(opts.Firmware.Flash)
		}

		if err := vmc.Save(); err != nil {
			return err
		}

		success = true

		return s.SystemdEnableService(vmname)
	})

	if err != nil {
		return nil, fmt.Errorf("cannot create new machine: %w", err)
	}

	return s.MachineGet(vmname, false)
}

func (s *Server) Delete(ctx context.Context, vmname string, force bool) error {
	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		if _, err := kvmrun.GetInstanceConf(vmname); err != nil {
			return err
		}

		if force {
			s.SystemdSendSIGKILL(vmname)
		}

		if err := s.SystemdStopService(vmname, 30*time.Second); err != nil {
			l.Errorf("Failed to shutdown %s: %s", vmname, err)
		}

		if err := s.SystemdDisableService(vmname); err != nil {
			return err
		}

		osuser.RemoveUser(vmname)

		if err := os.RemoveAll(filepath.Join(kvmrun.CONFDIR, vmname)); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("cannot delete machine: %w", err)
	}

	return nil
}

func (s *Server) FirmwareSet(ctx context.Context, vmname string, opts *kvmrun.FirmwareProperties, qemuRootDir string) error {
	if opts == nil {
		return fmt.Errorf("empty firmware opts")
	} else {
		if err := opts.Validate(false); err != nil {
			return err
		}
	}

	qemuRootDir = strings.TrimSpace(qemuRootDir)

	if len(qemuRootDir) == 0 {
		qemuRootDir = s.AppConf.Kvmrun.QemuRootDir
	}

	vmdir := filepath.Join(kvmrun.CONFDIR, vmname)

	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if len(opts.Flash) > 0 {
			if fi, err := os.Stat(opts.Flash); err == nil {
				if fi.IsDir() {
					return fmt.Errorf("not a file: %s", opts.Flash)
				}
			} else {
				if os.IsNotExist(err) {
					return err
				}
				opts.Flash = ""
			}
		}

		switch opts.Image {
		case "bios", "legacy":
			opts.Image = ""
			opts.Flash = ""
		case "efi", "uefi", "ovmf":
			possibleDirs := []string{
				filepath.Join(qemuRootDir, "usr/share/OVMF"),
				filepath.Join(qemuRootDir, "usr/share/ovmf"),
				filepath.Join(qemuRootDir, "usr/share/qemu"),
			}

			// Copy OVMF_CODE.fd to a virt.machine config dir
			_, fname, err := utils.LookForFile("OVMF_CODE.fd", possibleDirs...)
			if err != nil {
				return err
			}
			l.WithField("file", filepath.Base(fname)).Infof("Found at %s", fname)

			if err := fsutil.Copy(fname, filepath.Join(vmdir, "config_eficode")); err != nil {
				return fmt.Errorf("failed to copy config_eficode: %w", err)
			}
			l.WithField("file", filepath.Base(fname)).Infof("Copy to %s", filepath.Join(vmdir, "config_eficode"))

			opts.Image = filepath.Join(vmdir, "config_eficode")

			if len(opts.Flash) == 0 {
				err := func() error {
					_, fname, err := utils.LookForFile("OVMF_VARS.fd", possibleDirs...)
					if err != nil {
						return err
					}
					l.WithField("file", filepath.Base(fname)).Infof("Found at %s", fname)

					if err := fsutil.Copy(fname, filepath.Join(vmdir, "config_efivars")); err != nil {
						return fmt.Errorf("failed to copy config_efivars: %w", err)
					}
					l.WithField("file", filepath.Base(fname)).Infof("Copy to %s", filepath.Join(vmdir, "config_efivars"))

					return nil
				}()

				if err != nil && !os.IsExist(err) {
					return err
				}

				opts.Flash = filepath.Join(vmdir, "config_efivars")
			}
		}

		vm.C.FirmwareSetImage(opts.Image)
		vm.C.FirmwareSetFlash(opts.Flash)

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot set firmware: %w", err)
	}

	return nil
}

func (s *Server) FirmwareRemove(ctx context.Context, vmname string) error {
	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if err := vm.C.FirmwareRemoveConf(); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot remove firmware: %w", err)
	}

	return nil
}

func (s *Server) ExternalKernelSet(ctx context.Context, vmname string, opts *kvmrun.ExtKernelProperties) error {
	if opts == nil {
		return fmt.Errorf("empty external kernel opts")
	} else {
		if err := opts.Validate(true); err != nil {
			return err
		}
	}

	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		var savingRequire bool

		if len(opts.Image) > 0 {
			if err := vm.C.KernelSetImage(opts.Image); err != nil {
				return err
			}
			savingRequire = true
		}

		if len(opts.Cmdline) > 0 {
			if err := vm.C.KernelSetCmdline(opts.Cmdline); err != nil {
				return err
			}
			savingRequire = true
		}

		if len(opts.Initrd) > 0 {
			if err := vm.C.KernelSetInitrd(opts.Initrd); err != nil {
				return err
			}
			savingRequire = true
		}

		if len(opts.Modiso) > 0 {
			if err := vm.C.KernelSetModiso(opts.Modiso); err != nil {
				return err
			}
			savingRequire = true
		}

		if savingRequire {
			return vm.C.Save()
		}

		return nil

	})

	if err != nil {
		return fmt.Errorf("cannot update external kernel properties: %w", err)
	}

	return nil
}

func (s *Server) ExternalKernelRemove(ctx context.Context, vmname string) error {
	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if err := vm.C.KernelRemoveConf(); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot update external kernel properties: %w", err)
	}

	return nil
}
