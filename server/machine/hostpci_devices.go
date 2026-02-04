package machine

import (
	"context"
	"fmt"

	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

func (s *Server) HostDeviceAttach(ctx context.Context, vmname string, opts *kvmrun.HostDeviceProperties, strict bool) error {
	if opts == nil {
		return fmt.Errorf("empty host-PCI-device opts")
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

		if err := vm.C.HostDeviceAppend(*opts); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot attach host-PCI-device: %w", err)
	}

	return nil
}

func (s *Server) HostDeviceDetach(ctx context.Context, vmname, hexaddr string) error {
	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if err := vm.C.HostDeviceRemove(hexaddr); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot detach host-PCI-device: %w", err)
	}

	return nil
}

func (s *Server) HostDeviceSetMultifunctionOption(ctx context.Context, vmname, hexaddr string, enabled bool) error {
	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if err := vm.C.HostDeviceSetMultifunctionOption(hexaddr, enabled); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot change host-PCI-device multifunction option: %w", err)
	}

	return nil
}

func (s *Server) HostDeviceSetPrimaryGpuOption(ctx context.Context, vmname, hexaddr string, enabled bool) error {
	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if err := vm.C.HostDeviceSetPrimaryGPUOption(hexaddr, enabled); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot change host-PCI-device primaryGPU option: %w", err)
	}

	return nil
}
