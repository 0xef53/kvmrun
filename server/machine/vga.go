package machine

import (
	"context"
	"fmt"

	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

func (s *Server) VgaDeviceSetType(ctx context.Context, vmname string, opts *kvmrun.VgaDeviceProperties) error {
	if opts == nil {
		return fmt.Errorf("empty VGA opts")
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

		if err := vm.C.VgaDeviceSetType(opts.Type); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot set VGA type: %w", err)
	}

	return nil
}
