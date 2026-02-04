package machine

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

func (s *Server) DiskAttach(ctx context.Context, vmname string, opts *kvmrun.DiskProperties, position int, live bool) error {
	if opts == nil {
		return fmt.Errorf("empty disk opts")
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

			if err := vm.R.DiskAppend(*opts); err != nil {
				if kvmrun.IsAlreadyConnectedError(err) {
					// In this case just re-set the io limits
					if err := vm.R.DiskSetReadIops(opts.Path, opts.IopsRd); err != nil {
						return err
					}
					if err := vm.R.DiskSetWriteIops(opts.Path, opts.IopsWr); err != nil {
						return err
					}
				} else {
					return err
				}
			}
		}

		addToConf := func() error {
			if position >= 0 {
				return vm.C.DiskInsert(*opts, position)
			}
			return vm.C.DiskAppend(*opts)
		}

		if err := addToConf(); err != nil {
			if kvmrun.IsAlreadyConnectedError(err) {
				// In this case just re-set the io limits
				if err := vm.C.DiskSetReadIops(opts.Path, opts.IopsRd); err != nil {
					return err
				}
				if err := vm.C.DiskSetWriteIops(opts.Path, opts.IopsWr); err != nil {
					return err
				}
			} else {
				return err
			}
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot attach or update disk: %w", err)
	}

	return nil
}

func (s *Server) DiskDetach(ctx context.Context, vmname, diskname string, live bool) error {
	diskname = strings.TrimSpace(diskname)

	if len(diskname) == 0 {
		return fmt.Errorf("empty disk name")
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if err := vm.R.DiskRemove(diskname); err != nil && !kvmrun.IsNotConnectedError(err) {
				return err
			}
		}

		if err := vm.C.DiskRemove(diskname); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot detach disk: %w", err)
	}

	return nil
}

func (s *Server) DiskSetReadLimit(ctx context.Context, vmname, diskname string, limit int, live bool) error {
	diskname = strings.TrimSpace(diskname)

	if len(diskname) == 0 {
		return fmt.Errorf("empty disk name")
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if err := vm.R.DiskSetReadIops(diskname, limit); err != nil {
				return err
			}
		}

		if err := vm.C.DiskSetReadIops(diskname, limit); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot update disk read limit: %w", err)
	}

	return nil
}

func (s *Server) DiskSetWriteLimit(ctx context.Context, vmname, diskname string, limit int, live bool) error {
	diskname = strings.TrimSpace(diskname)

	if len(diskname) == 0 {
		return fmt.Errorf("empty disk name")
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if err := vm.R.DiskSetWriteIops(diskname, limit); err != nil {
				return err
			}
		}

		if err := vm.C.DiskSetWriteIops(diskname, limit); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot update disk read limit: %w", err)
	}

	return nil
}

func (s *Server) DiskRemoveQemuBitmap(ctx context.Context, vmname, diskname string) error {
	diskname = strings.TrimSpace(diskname)

	if len(diskname) == 0 {
		return fmt.Errorf("empty disk name")
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname+":"+diskname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if vm.R != nil {
			return vm.R.DiskRemoveQemuBitmap(diskname)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("cannot remove disk bitmap: %w", err)
	}

	return nil
}

func (s *Server) DiskResizeQemuBlockdev(ctx context.Context, vmname, diskname string) error {
	diskname = strings.TrimSpace(diskname)

	if len(diskname) == 0 {
		return fmt.Errorf("empty disk name")
	}

	err := s.TaskRunFunc(ctx, server.NoBlockOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if vm.R != nil {
			return vm.R.DiskResizeQemuBlockdev(diskname)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("cannot send block-resize event: %w", err)
	}

	return nil
}
