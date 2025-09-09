package machine

import (
	"context"
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

func (s *Server) NetIfaceAttach(ctx context.Context, vmname string, opts *kvmrun.NetIfaceProperties, live bool) error {
	if opts == nil {
		return fmt.Errorf("empty network interface opts")
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
			if err := vm.R.NetIfaceAppend(*opts); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
				return err
			}
		}

		if err := vm.C.NetIfaceAppend(*opts); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot attach network interface: %w", err)
	}

	return nil
}

func (s *Server) NetIfaceDetach(ctx context.Context, vmname, ifname string, live bool) error {
	ifname = strings.TrimSpace(ifname)

	if len(ifname) == 0 {
		return fmt.Errorf("empty interface name")
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if live && vm.R != nil {
			if err := vm.R.NetIfaceRemove(ifname); err != nil && !kvmrun.IsNotConnectedError(err) {
				return err
			}
		}

		if err := vm.C.NetIfaceRemove(ifname); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot detach network interface: %w", err)
	}

	return nil
}

func (s *Server) NetIfaceSetLinkState(ctx context.Context, vmname, ifname string, state kvmrun.NetLinkState) error {
	ifname = strings.TrimSpace(ifname)

	if len(ifname) == 0 {
		return fmt.Errorf("empty interface name")
	}

	err := s.TaskRunFunc(ctx, server.NoBlockOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if vm.R == nil {
			return fmt.Errorf("not running: %s", vmname)
		}

		var set func(string) error

		switch state {
		case kvmrun.NetLinkState_UP:
			set = vm.R.NetIfaceSetLinkUp
		case kvmrun.NetLinkState_DOWN:
			set = vm.R.NetIfaceSetLinkDown
		default:
			return fmt.Errorf("incorrect link state: %d", state)
		}

		return set(ifname)
	})

	if err != nil {
		return fmt.Errorf("cannot set network interface link state: %w", err)
	}

	return nil
}

func (s *Server) NetIfaceSetUpScript(ctx context.Context, vmname, ifname, script string) error {
	ifname = strings.TrimSpace(ifname)

	if len(ifname) == 0 {
		return fmt.Errorf("empty interface name")
	}

	script = strings.TrimSpace(script)

	if len(script) == 0 {
		return fmt.Errorf("empty script path")
	}

	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if err := vm.C.NetIfaceSetUpScript(ifname, script); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot set network interface if-up script: %w", err)
	}

	return nil
}

func (s *Server) NetIfaceSetDownScript(ctx context.Context, vmname, ifname, script string) error {
	ifname = strings.TrimSpace(ifname)

	if len(ifname) == 0 {
		return fmt.Errorf("empty interface name")
	}

	script = strings.TrimSpace(script)

	if len(script) == 0 {
		return fmt.Errorf("empty script path")
	}

	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if err := vm.C.NetIfaceSetDownScript(ifname, script); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot set network interface if-down script: %w", err)
	}

	return nil
}

func (s *Server) NetIfaceSetQueues(ctx context.Context, vmname, ifname string, queues int) error {
	ifname = strings.TrimSpace(ifname)

	if len(ifname) == 0 {
		return fmt.Errorf("empty interface name")
	}

	err := s.TaskRunFunc(ctx, server.BlockConfOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, false)
		if err != nil {
			return err
		}

		if err := vm.C.NetIfaceSetQueues(ifname, queues); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return fmt.Errorf("cannot set network interface queues: %w", err)
	}

	return nil
}
