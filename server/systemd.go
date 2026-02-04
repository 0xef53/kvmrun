package server

import (
	"time"

	"github.com/0xef53/kvmrun/internal/systemd"
)

func (s *Server) SystemdGetUnit(vmname string) (*systemd.UnitStatus, error) {
	return s.SystemCtl.GetUnit(machineUnitFile(vmname))
}

func (s *Server) SystemdEnableService(vmname string) error {
	return s.SystemCtl.Enable(machineUnitFile(vmname))
}

func (s *Server) SystemdDisableService(vmname string) error {
	return s.SystemCtl.Disable(machineUnitFile(vmname))
}

func (s *Server) SystemdStartService(vmname string, interval time.Duration) error {
	if interval == 0 {
		return s.SystemCtl.Start(machineUnitFile(vmname), nil)
	}

	return s.SystemCtl.StartAndTest(machineUnitFile(vmname), interval, nil)
}

func (s *Server) SystemdRestartService(vmname string) error {
	return s.SystemCtl.Restart(machineUnitFile(vmname), nil)
}

func (s *Server) SystemdStopService(vmname string, interval time.Duration) error {
	if interval == 0 {
		return s.SystemCtl.Stop(machineUnitFile(vmname), nil)
	}

	return s.SystemCtl.StopAndWait(machineUnitFile(vmname), interval, nil)
}

func (s *Server) SystemdSendSIGTERM(vmname string) {
	s.SystemCtl.KillBySIGTERM(machineUnitFile(vmname))
}

func (s *Server) SystemdSendSIGKILL(vmname string) {
	s.SystemCtl.KillBySIGKILL(machineUnitFile(vmname))
}
