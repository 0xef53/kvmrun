package machines

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/internal/fsutil"
	"github.com/0xef53/kvmrun/internal/helpers"
	"github.com/0xef53/kvmrun/internal/osuser"
	"github.com/0xef53/kvmrun/kvmrun"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

func (s *ServiceServer) Create(ctx context.Context, req *pb.CreateMachineRequest) (*pb.CreateMachineResponse, error) {
	allowedNames := regexp.MustCompile(`^[0-9A-Za-z_]{3,16}$`)

	if !allowedNames.MatchString(req.Name) {
		return nil, fmt.Errorf("invalid machine name: only [0-9A-Za-z_] are allowed, min length is 3 and max length is 16")
	}

	var mproto *pb_types.Machine

	vmdir := filepath.Join(kvmrun.CONFDIR, req.Name)
	vmlogdir := filepath.Join(kvmrun.LOGDIR, req.Name)

	if req.Options.Firmware != nil {
		req.Options.Firmware.Image = strings.TrimSpace(req.Options.Firmware.Image)
		req.Options.Firmware.Flash = strings.TrimSpace(req.Options.Firmware.Flash)
	}

	err := s.RunFuncTask(ctx, req.Name+"::", func(l *log.Entry) error {
		if _, err := os.Stat(filepath.Join(vmdir, "config")); err == nil {
			return grpc_status.Errorf(grpc_codes.AlreadyExists, "already exists: %s", req.Name)
		}

		var success bool

		for _, d := range []string{vmdir, vmlogdir} {
			if err := os.MkdirAll(d, 0755); err != nil {
				return err
			}
		}
		defer func() {
			if !success {
				for _, d := range []string{vmdir, vmlogdir} {
					os.RemoveAll(d)
				}
			}
		}()

		if req.Options.Firmware != nil {
			if len(req.Options.Firmware.Flash) > 0 {
				if fi, err := os.Stat(req.Options.Firmware.Flash); err == nil {
					if fi.IsDir() {
						return fmt.Errorf("not a file: %s", req.Options.Firmware.Flash)
					}
				} else {
					if os.IsNotExist(err) {
						return err
					}
					req.Options.Firmware.Flash = ""
				}
			}

			switch req.Options.Firmware.Image {
			case "bios", "legacy":
				req.Options.Firmware.Image = ""
				req.Options.Firmware.Flash = ""
			case "efi", "uefi", "ovmf":
				qemuRootDir := s.AppConf.Common.QemuRootDir

				if v := strings.TrimSpace(req.QemuRootdir); len(v) != 0 {
					qemuRootDir = v
				}

				possibleDirs := []string{
					filepath.Join(qemuRootDir, "usr/share/OVMF"),
					filepath.Join(qemuRootDir, "usr/share/ovmf"),
					filepath.Join(qemuRootDir, "usr/share/qemu"),
				}

				// Copy OVMF_CODE.fd to a virt.machine config dir
				_, fname, err := helpers.LookForFile("OVMF_CODE.fd", possibleDirs...)
				if err != nil {
					return err
				}
				fmt.Printf("DEBUG(create) Found CODE by LookForFile at %s\n", fname)

				if err := fsutil.Copy(fname, filepath.Join(vmdir, "config_eficode")); err != nil {
					return fmt.Errorf("failed to copy config_eficode: %w", err)
				}
				fmt.Printf("DEBUG(create) Copy from %s to %s\n", fname, filepath.Join(vmdir, "config_eficode"))

				req.Options.Firmware.Image = fname

				// Copy OVMF_VARS.fd to a virt.machine config dir
				if len(req.Options.Firmware.Flash) == 0 {
					err := func() error {
						_, fname, err := helpers.LookForFile("OVMF_VARS.fd", possibleDirs...)
						if err != nil {
							return err
						}
						fmt.Printf("DEBUG(create) Found VARS by LookForFile at %s\n", fname)

						if err := fsutil.Copy(fname, filepath.Join(vmdir, "config_efivars")); err != nil {
							return fmt.Errorf("failed to copy config_efivars: %w", err)
						}
						fmt.Printf("DEBUG(create) Copy from %s to %s\n", fname, filepath.Join(vmdir, "config_efivars"))

						return nil
					}()

					if err != nil && !os.IsExist(err) {
						return err
					}

					req.Options.Firmware.Flash = filepath.Join(vmdir, "config_efivars")
				}
			}
		}

		if _, err := osuser.CreateUser(req.Name); err != nil {
			return err
		}

		success = true

		vmc := kvmrun.NewInstanceConf(req.Name)

		vmc.SetTotalMem(int(req.Options.Memory.Total))
		vmc.SetActualMem(int(req.Options.Memory.Actual))
		vmc.SetTotalCPUs(int(req.Options.CPU.Total))
		vmc.SetActualCPUs(int(req.Options.CPU.Actual))
		vmc.SetCPUQuota(int(req.Options.CPU.Quota))
		vmc.SetCPUModel(req.Options.CPU.Model)

		if req.Options.Firmware != nil {
			vmc.SetFirmwareImage(req.Options.Firmware.Image)
			vmc.SetFirmwareFlash(req.Options.Firmware.Flash)
		}

		if err := vmc.Save(); err != nil {
			return err
		}

		mproto = machineToProto(&kvmrun.Machine{Name: req.Name, C: vmc}, kvmrun.StateInactive, 0)

		return s.SystemCtl.Enable(s.MachineToUnit(req.Name))
	})

	if err != nil {
		return nil, err
	}

	return &pb.CreateMachineResponse{Machine: mproto}, nil
}

func (s *ServiceServer) Delete(ctx context.Context, req *pb.DeleteMachineRequest) (*pb.DeleteMachineResponse, error) {
	var mproto *pb_types.Machine

	err := s.RunFuncTask(ctx, req.Name+"::", func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.Force {
			s.SystemCtl.KillBySIGKILL(s.MachineToUnit(req.Name))
		}

		if err := s.SystemCtl.StopAndWait(s.MachineToUnit(req.Name), 30*time.Second, nil); err != nil {
			l.Errorf("Failed to shutdown %s: %s", req.Name, err)
		}

		if err := s.SystemCtl.Disable(s.MachineToUnit(req.Name)); err != nil {
			return err
		}

		osuser.RemoveUser(req.Name)

		dirs := []string{
			filepath.Join(kvmrun.CONFDIR, req.Name),
			filepath.Join(kvmrun.LOGDIR, req.Name),
		}
		for _, d := range dirs {
			if err := os.RemoveAll(d); err != nil {
				return err
			}
		}

		mproto = machineToProto(vm, kvmrun.StateNoState, 0)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &pb.DeleteMachineResponse{Machine: mproto}, nil
}

func (s *ServiceServer) Get(ctx context.Context, req *pb.GetMachineRequest) (*pb.GetMachineResponse, error) {
	vm, err := s.GetMachine(req.Name)
	if err != nil {
		return nil, err
	}

	vmstate, err := s.GetMachineStatus(vm)
	if err != nil {
		return nil, err
	}

	return &pb.GetMachineResponse{Machine: machineToProto(vm, vmstate, 0)}, nil
}

func (s *ServiceServer) GetEvents(ctx context.Context, req *pb.GetMachineRequest) (*pb.GetEventsResponse, error) {
	events, found, err := s.Mon.FindEvents(req.Name, "", 0)
	if err == nil {
		if found {
			return &pb.GetEventsResponse{Events: eventsToProto(events)}, nil
		}
	} else {
		if _, ok := err.(*net.OpError); !ok {
			return nil, err
		}
	}

	return new(pb.GetEventsResponse), nil
}

func (s *ServiceServer) List(ctx context.Context, req *pb.ListMachinesRequest) (*pb.ListMachinesResponse, error) {
	var names []string

	allNames, err := s.GetMachineNames()
	if err != nil {
		return nil, err
	}

	if len(req.Names) == 0 {
		names = allNames
	} else {
		names = make([]string, 0, len(req.Names))

		for _, n := range req.Names {
			if stringSliceContains(allNames, n) {
				names = append(names, n)
			}
		}
	}

	if len(names) == 0 {
		return &pb.ListMachinesResponse{}, nil
	}

	get := func(name string) *pb_types.Machine {
		vm, err := s.GetMachine(name)
		if err != nil {
			fmt.Println("LIST ERR 1: ", err.Error())
			return &pb_types.Machine{Name: name, State: pb_types.MachineState_CRASHED}
		}
		vmstate, err := s.GetMachineStatus(vm)
		if err != nil {
			fmt.Println("LIST ERR 2: ", err.Error())
			return &pb_types.Machine{Name: name, State: pb_types.MachineState_CRASHED}
		}
		t, err := s.GetMachineLifeTime(vm)
		if err != nil {
			fmt.Println("LIST ERR 3: ", err.Error())
			return &pb_types.Machine{Name: name, State: pb_types.MachineState_CRASHED}
		}
		return machineToProto(vm, vmstate, t)
	}

	vmlist := make([]*pb_types.Machine, 0, len(names))

	results := make(chan *pb_types.Machine, runtime.NumCPU())

	go func() {
		jobs := make(chan string)

		// Run workers
		for i := 0; i < runtime.NumCPU(); i++ {
			go func() {
				for name := range jobs {
					results <- get(name)
				}
			}()
		}

		for _, name := range names {
			jobs <- name
		}

		close(jobs)
	}()

	for i := 0; i < len(names); i++ {
		vmlist = append(vmlist, <-results)
	}
	close(results)

	// Sort by virtual machine name
	sort.Slice(vmlist, func(i, j int) bool {
		return vmlist[i].Name < vmlist[j].Name
	})

	return &pb.ListMachinesResponse{Machines: vmlist}, nil
}

func (s *ServiceServer) ListNames(ctx context.Context, req *pb.ListMachinesRequest) (*pb.ListNamesResponse, error) {
	var names []string

	allNames, err := s.GetMachineNames()
	if err != nil {
		return nil, err
	}

	if len(req.Names) == 0 {
		names = allNames
	} else {
		names = make([]string, 0, len(req.Names))

		for _, n := range req.Names {
			if stringSliceContains(allNames, n) {
				names = append(names, n)
			}
		}
	}

	return &pb.ListNamesResponse{Machines: names}, nil
}

func (s *ServiceServer) SetFirmware(ctx context.Context, req *pb.SetFirmwareRequest) (*empty.Empty, error) {
	req.Image = strings.TrimSpace(req.Image)
	req.Flash = strings.TrimSpace(req.Flash)

	vmdir := filepath.Join(kvmrun.CONFDIR, req.Name)

	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.RemoveConf {
			if err := vm.C.RemoveFirmwareConf(); err != nil {
				return err
			}
			return vm.C.Save()
		}

		if len(req.Flash) > 0 {
			if fi, err := os.Stat(req.Flash); err == nil {
				if fi.IsDir() {
					return fmt.Errorf("not a file: %s", req.Flash)
				}
			} else {
				if os.IsNotExist(err) {
					return err
				}
				req.Flash = ""
			}
		}

		switch req.Image {
		case "bios", "legacy":
			req.Image = ""
			req.Flash = ""
		case "efi", "uefi", "ovmf":
			qemuRootDir := s.AppConf.Common.QemuRootDir

			if v := strings.TrimSpace(req.QemuRootdir); len(v) != 0 {
				qemuRootDir = v
			}

			possibleDirs := []string{
				filepath.Join(qemuRootDir, "usr/share/OVMF"),
				filepath.Join(qemuRootDir, "usr/share/ovmf"),
				filepath.Join(qemuRootDir, "usr/share/qemu"),
			}

			// Copy OVMF_CODE.fd to a virt.machine config dir
			_, fname, err := helpers.LookForFile("OVMF_CODE.fd", possibleDirs...)
			if err != nil {
				return err
			}
			fmt.Printf("DEBUG(set-firmware) Found CODE by LookForFile at %s\n", fname)

			if err := fsutil.Copy(fname, filepath.Join(vmdir, "config_eficode")); err != nil {
				return fmt.Errorf("failed to copy config_eficode: %w", err)
			}
			fmt.Printf("DEBUG(set-firmware) Copy from %s to %s\n", fname, filepath.Join(vmdir, "config_eficode"))

			req.Image = fname

			if len(req.Flash) == 0 {
				err := func() error {
					_, fname, err := helpers.LookForFile("OVMF_VARS.fd", possibleDirs...)
					if err != nil {
						return err
					}
					fmt.Printf("DEBUG(set-firmware) Found VARS by LookForFile at %s\n", fname)

					if err := fsutil.Copy(fname, filepath.Join(vmdir, "config_efivars")); err != nil {
						return fmt.Errorf("failed to copy config_efivars: %w", err)
					}
					fmt.Printf("DEBUG(set-firmware) Copy from %s to %s\n", fname, filepath.Join(vmdir, "config_efivars"))

					return nil
				}()

				if err != nil && !os.IsExist(err) {
					return err
				}

				req.Flash = filepath.Join(vmdir, "config_efivars")
			}
		}

		vm.C.SetFirmwareImage(req.Image)
		vm.C.SetFirmwareFlash(req.Flash)

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
