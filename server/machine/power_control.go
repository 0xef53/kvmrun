package machine

import (
	"context"
	"time"

	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

func (s *Server) Start(ctx context.Context, vmname string, waitTime time.Duration) error {
	if _, err := kvmrun.GetInstanceConf(vmname); err != nil {
		return err
	}

	return s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		s.MachineRemoveDownFile(vmname)

		return s.SystemdStartService(vmname, waitTime)
	})
}

func (s *Server) Stop(ctx context.Context, vmname string, wait, force bool) error {
	if _, err := kvmrun.GetInstanceConf(vmname); err != nil {
		return err
	}

	return s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		if err := s.MachineCreateDownFile(vmname); err != nil {
			return err
		}

		if force {
			s.SystemdSendSIGTERM(vmname)
		}

		if wait {
			return s.SystemdStopService(vmname, 60*time.Second)
		}

		return s.SystemdStopService(vmname, 0)
	})
}

func (s *Server) Restart(ctx context.Context, vmname string, wait bool) error {
	if _, err := kvmrun.GetInstanceConf(vmname); err != nil {
		return err
	}

	return s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		s.MachineRemoveDownFile(vmname)

		if wait {
			if err := s.SystemdStopService(vmname, 60*time.Second); err != nil {
				return err
			}
			return s.SystemdStartService(vmname, 0)
		}

		return s.SystemdRestartService(vmname)
	})
}

func (s *Server) Reset(ctx context.Context, vmname string) error {
	if _, err := kvmrun.GetInstanceConf(vmname); err != nil {
		return err
	}

	return s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname), true, nil, func(l *log.Entry) error {
		s.MachineRemoveDownFile(vmname)

		s.SystemdSendSIGKILL(vmname)

		if err := s.SystemdStopService(vmname, 30*time.Second); err != nil {
			log.Errorf("Failed to shutdown %s: %s", vmname, err)
		}

		return s.SystemdStartService(vmname, 0)
	})
}
