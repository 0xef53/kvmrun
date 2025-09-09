package machine

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

func (s *Server) InputDeviceAttach(ctx context.Context, vmname string, opts *kvmrun.InputDeviceProperties) error {
	if opts == nil {
		return fmt.Errorf("empty input device opts")
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

		if err := vm.C.InputDeviceAppend(*opts); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot attach input device: %w", err)
	}

	return nil
}

func (s *Server) InputDeviceDetach(ctx context.Context, vmname, deviceType string) error {
	deviceType = strings.TrimSpace(deviceType)

	if len(deviceType) == 0 {
		return fmt.Errorf("empty device type")
	}

	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if err := vm.C.InputDeviceRemove(deviceType); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot detach input device: %w", err)
	}

	return nil
}
