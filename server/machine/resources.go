package machine

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

func (s *Server) MemorySetLimits(ctx context.Context, vmname string, actual, total int, live bool) error {
	set := func(vmi kvmrun.Instance) error {
		if total == 0 {
			total = vmi.MemoryGetTotal()
		}

		if actual > vmi.MemoryGetTotal() {
			if err := vmi.MemorySetTotal(total); err != nil && err != kvmrun.ErrNotImplemented {
				return err
			}
			if err := vmi.MemorySetActual(actual); err != nil {
				return err
			}
		} else {
			if err := vmi.MemorySetActual(actual); err != nil {
				return err
			}
			if err := vmi.MemorySetTotal(total); err != nil && err != kvmrun.ErrNotImplemented {
				return err
			}
		}

		return nil
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if total > 0 && total != vm.R.MemoryGetTotal() {
				return fmt.Errorf("unable to change total memory while the machine is running")
			}

			if err := set(vm.R); err != nil {
				return err
			}
		}

		if err := set(vm.C); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot set memory limits: %w", err)
	}

	return nil
}

func (s *Server) CPUSetLimits(ctx context.Context, vmname string, actual, total int, live bool) error {
	set := func(vmi kvmrun.Instance) error {
		if total == 0 {
			total = vmi.CPUGetTotal()
		}

		if actual > vmi.CPUGetTotal() {
			if err := vmi.CPUSetTotal(total); err != nil && err != kvmrun.ErrNotImplemented {
				return err
			}
			if err := vmi.CPUSetActual(actual); err != nil {
				return err
			}
		} else {
			if err := vmi.CPUSetActual(actual); err != nil {
				return err
			}
			if err := vmi.CPUSetTotal(total); err != nil && err != kvmrun.ErrNotImplemented {
				return err
			}
		}

		return nil
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if total > 0 && total != vm.R.CPUGetTotal() {
				return fmt.Errorf("unable to change total vCPU count while the machine is running")
			}

			if err := set(vm.R); err != nil {
				return err
			}
		}

		if err := set(vm.C); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot set vCPU limits: %w", err)
	}

	return nil
}

func (s *Server) CPUSetSockets(ctx context.Context, vmname string, count int) error {
	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if err := vm.C.CPUSetSockets(count); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot set vCPU socket count: %w", err)
	}

	return nil
}

func (s *Server) CPUSetQuota(ctx context.Context, vmname string, quota int, live bool) error {
	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if err := vm.R.CPUSetQuota(quota); err != nil {
				return err
			}
		}

		if err := vm.C.CPUSetQuota(quota); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot set vCPU quota: %w", err)
	}

	return nil
}

func (s *Server) CPUSetModel(ctx context.Context, vmname, model string) error {
	model = strings.TrimSpace(model)

	if len(model) == 0 {
		return fmt.Errorf("empty model name")
	}

	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if err := vm.C.CPUSetModel(model); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot set vCPU model: %w", err)
	}

	return nil
}
