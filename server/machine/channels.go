package machine

import (
	"context"
	"fmt"

	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

func (s *Server) ChannelAttach_VSock(ctx context.Context, vmname string, opts *kvmrun.ChannelVSockProperties, live bool) error {
	if opts == nil {
		return fmt.Errorf("empty virtio-vsock opts")
	} else {
		if err := opts.Validate(true); err != nil {
			return err
		}
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if err := vm.R.VSockDeviceAppend(*opts); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
				return err
			}
		}

		if err := vm.C.VSockDeviceAppend(*opts); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot attach virtio-vsock channel: %w", err)
	}

	return nil
}

func (s *Server) ChannelDetach_VSock(ctx context.Context, vmname string, live bool) error {
	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if err := vm.R.VSockDeviceRemove(); err != nil && !kvmrun.IsNotConnectedError(err) {
				return err
			}
		}

		if err := vm.C.VSockDeviceRemove(); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot detach virtio-vsock channel: %w", err)
	}

	return nil
}

func (s *Server) ChannelAttach_SerialPort(_ context.Context, _ string, _ bool) error {
	return fmt.Errorf("cannot attach serial-port channel: %w", kvmrun.ErrNotImplemented)
}

func (s *Server) ChannelDetach_SerialPort(_ context.Context, _ string, _ bool) error {
	return fmt.Errorf("cannot detach serial-port channel: %w", kvmrun.ErrNotImplemented)
}
