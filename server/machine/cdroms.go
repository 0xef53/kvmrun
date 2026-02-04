package machine

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

func (s *Server) CdromAttach(ctx context.Context, vmname string, opts *kvmrun.CdromProperties, position int, live bool) error {
	if opts == nil {
		return fmt.Errorf("empty cdrom opts")
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
			// -1 -- означает, что параметр не нужно брать в расчет
			if position >= 0 {
				return fmt.Errorf("unable to insert at the '%d' position while the machine is running", position)
			}

			if err := vm.R.CdromAppend(*opts); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
				return err
			}
		}

		addToConf := func() error {
			if position >= 0 {
				return vm.C.CdromInsert(*opts, position)
			}
			return vm.C.CdromAppend(*opts)
		}

		if err := addToConf(); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot attach cdrom: %w", err)
	}

	return nil
}

func (s *Server) CdromDetach(ctx context.Context, vmname, devname string, live bool) error {
	devname = strings.TrimSpace(devname)

	if len(devname) == 0 {
		return fmt.Errorf("empty cdrom name")
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if err := vm.R.CdromRemove(devname); err != nil && !kvmrun.IsNotConnectedError(err) {
				return err
			}
		}

		if err := vm.C.CdromRemove(devname); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot detach cdrom: %w", err)
	}

	return nil
}

func (s *Server) CdromChangeMedia(ctx context.Context, vmname, devname, media string, live bool) error {
	devname = strings.TrimSpace(devname)

	if len(devname) == 0 {
		return fmt.Errorf("empty cdrom name")
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if err := vm.R.CdromChangeMedia(devname, media); err != nil && !kvmrun.IsNotConnectedError(err) {
				return err
			}
		}

		if err := vm.C.CdromChangeMedia(devname, media); err != nil {
			if kvmrun.IsAlreadyConnectedError(err) {
				// Nothing to do
				return nil
			}
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot update cdrom media: %w", err)
	}

	return nil
}

func (s *Server) CdromRemoveMedia(ctx context.Context, vmname, devname string, live bool) error {
	devname = strings.TrimSpace(devname)

	if len(devname) == 0 {
		return fmt.Errorf("empty cdrom name")
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if err := vm.R.CdromRemoveMedia(devname); err != nil {
				return err
			}
		}

		if err := vm.C.CdromRemoveMedia(devname); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot remove cdrom media: %w", err)
	}

	return nil
}
