package machine

import (
	"context"
	"fmt"

	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

func (s *Server) CloudInitDriveAttach(ctx context.Context, vmname string, opts *kvmrun.CloudInitDriveProperties) error {
	if opts == nil {
		return fmt.Errorf("empty cloud-init opts")
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

		if err := vm.C.CloudInitSetMedia(opts.Media); err != nil {
			return err
		}
		if err := vm.C.CloudInitSetDriver(opts.Driver); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot attach cloud-init drive: %w", err)
	}

	return nil
}

func (s *Server) CloudInitDriveDetach(ctx context.Context, vmname string) error {
	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if err := vm.C.CloudInitRemoveConf(); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot detach cloud-init drive: %w", err)
	}

	return nil
}

func (s *Server) CloudInitDriveChangeMedia(ctx context.Context, vmname, media string, live bool) error {
	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if err := vm.R.CloudInitSetMedia(media); err != nil {
				return err
			}
		}

		if drive := vm.C.CloudInitGetDrive(); drive != nil {
			if media != drive.Media {
				if err := vm.C.CloudInitSetMedia(media); err != nil {
					return err
				}

				return vm.C.Save()
			}
		} else {
			return &kvmrun.NotConnectedError{Source: "instance_conf", Object: "cloud-init drive"}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("cannot update cloud-init media: %w", err)
	}

	return nil
}
