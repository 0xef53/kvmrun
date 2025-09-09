package server

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/internal/utils"
	"github.com/0xef53/kvmrun/kvmrun"

	qmp "github.com/0xef53/go-qmp/v2"
)

func (s *Server) MachineGet(name string, runtime bool) (*kvmrun.Machine, error) {
	get := func(name string, mon *qmp.Monitor) (*kvmrun.Machine, error) {
		vm, err := kvmrun.GetMachine(name, mon)

		if err != nil {
			if _, ok := err.(*net.OpError); ok {
				// If a QEMU process was terminated bypassing kvmrun
				// (for example: SIGTERM / SIGKILL / SIGINT) we will get
				// an error of type *net.OpError with code == syscall.ECONNRESET.
				// As a workaround for this case - just repeat the request with mon = nil
				return kvmrun.GetMachine(name, nil)
			}

			return nil, err
		}

		return vm, nil
	}

	if runtime {
		mon, _ := s.Mon.Get(name)

		return get(name, mon)
	}

	return get(name, nil)
}

func (s *Server) MachineGetStatus(vm *kvmrun.Machine) (kvmrun.InstanceState, error) {
	if vm == nil {
		return 0, fmt.Errorf("empty machine instance")
	}

	var vmi kvmrun.Instance

	if vm.R != nil {
		vmi = vm.R
	} else {
		vmi = vm.C
	}

	// Trying to get the special status of instance.
	// Exiting on success.
	if st, err := vmi.Status(); err == nil {
		switch st {
		case kvmrun.StatePaused:
			return st, nil
		case kvmrun.StateIncoming, kvmrun.StateMigrating, kvmrun.StateMigrated:
			return st, nil
		}
	} else {
		return kvmrun.StateNoState, err
	}

	unit, err := s.SystemdGetUnit(vm.Name)
	if err != nil {
		return kvmrun.StateNoState, fmt.Errorf("systemd dbus request failed: %w", err)
	}

	switch unit.ActiveState {
	case "active":
		switch unit.SubState {
		case "running":
			return kvmrun.StateRunning, nil
		}
	case "inactive":
		return kvmrun.StateInactive, nil
	case "activating":
		return kvmrun.StateStarting, nil
	case "deactivating":
		return kvmrun.StateShutdown, nil
	case "failed":
		return kvmrun.StateCrashed, nil
	}

	return kvmrun.StateNoState, nil
}

func (s *Server) MachineGetLifeTime(vm *kvmrun.Machine) (time.Duration, error) {
	if vm == nil {
		return 0, fmt.Errorf("empty machine instance")
	}

	if vm.R == nil {
		return 0, nil
	}

	t, err := utils.GetProcessLifeTime(vm.R.PID())
	if err != nil {
		return 0, err
	}

	return t, nil
}

func (s *Server) MachineGetNames(names ...string) ([]string, error) {
	files, err := os.ReadDir(kvmrun.CONFDIR)
	if err != nil {
		return nil, err
	}

	allNames := make([]string, 0, len(files))

	for _, f := range files {
		conffile := filepath.Join(kvmrun.CONFDIR, f.Name(), "config")

		if _, err := os.Stat(conffile); err == nil {
			allNames = append(allNames, f.Name())
		}
	}

	if len(names) == 0 {
		names = allNames
	} else {
		validNames := make([]string, 0, len(names))

		for _, n := range names {
			if slices.Contains(allNames, n) {
				validNames = append(validNames, n)
			}
		}

		names = validNames
	}

	return names, nil
}

func (s *Server) MachineGetEvents(vmname string) ([]qmp.Event, error) {
	vmname = strings.TrimSpace(vmname)

	if len(vmname) == 0 {
		return nil, fmt.Errorf("empty machine name")
	}

	events, found, err := s.Mon.FindEvents(vmname, "", 0)
	if err == nil {
		if found {
			return events, nil
		}
	} else {
		if _, ok := err.(*net.OpError); !ok {
			return nil, err
		}
	}

	return nil, nil
}

func machineDownFile(vmname string) string {
	return filepath.Join(filepath.Join(kvmrun.CONFDIR, vmname, "down"))
}

func machineUnitFile(vmname string) string {
	return "kvmrun@" + vmname + ".service"
}

func (s *Server) MachineCreateDownFile(vmname string) error {
	vmname = strings.TrimSpace(vmname)

	if len(vmname) == 0 {
		return fmt.Errorf("empty machine name")
	}

	return os.WriteFile(machineDownFile(vmname), []byte(""), 0644)
}

func (s *Server) MachineRemoveDownFile(vmname string) error {
	vmname = strings.TrimSpace(vmname)

	if len(vmname) == 0 {
		return fmt.Errorf("empty machine name")
	}

	return os.Remove(machineDownFile(vmname))
}
