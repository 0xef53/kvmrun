package machines

import (
	"context"
	"os"
	"time"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	"github.com/0xef53/kvmrun/kvmrun"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
)

func (s *ServiceServer) Start(ctx context.Context, req *pb.StartMachineRequest) (*empty.Empty, error) {
	if _, err := kvmrun.GetInstanceConf(req.Name); err != nil {
		return nil, err
	}

	err := s.RunFuncTask(ctx, req.Name+"::", func(l *log.Entry) error {
		os.Remove(s.MachineDownFile(req.Name))

		if req.WaitInterval > 0 {
			return s.SystemCtl.StartAndTest(s.MachineToUnit(req.Name), time.Duration(req.WaitInterval)*time.Second, nil)
		}

		return s.SystemCtl.Start(s.MachineToUnit(req.Name), nil)
	})

	return new(empty.Empty), err
}

func (s *ServiceServer) Stop(ctx context.Context, req *pb.StopMachineRequest) (*empty.Empty, error) {
	if _, err := kvmrun.GetInstanceConf(req.Name); err != nil {
		return nil, err
	}

	err := s.RunFuncTask(ctx, req.Name+"::", func(l *log.Entry) error {
		if err := os.WriteFile(s.MachineDownFile(req.Name), []byte(""), 0644); err != nil {
			return err
		}

		if req.Force {
			s.SystemCtl.KillBySIGTERM(s.MachineToUnit(req.Name))
		}

		if req.Wait {
			return s.SystemCtl.StopAndWait(s.MachineToUnit(req.Name), 60*time.Second, nil)
		}

		return s.SystemCtl.Stop(s.MachineToUnit(req.Name), nil)
	})

	return new(empty.Empty), err
}

func (s *ServiceServer) Restart(ctx context.Context, req *pb.RestartMachineRequest) (*empty.Empty, error) {
	if _, err := kvmrun.GetInstanceConf(req.Name); err != nil {
		return nil, err
	}

	err := s.RunFuncTask(ctx, req.Name+"::", func(l *log.Entry) error {
		os.Remove(s.MachineDownFile(req.Name))

		if req.Wait {
			if err := s.SystemCtl.StopAndWait(s.MachineToUnit(req.Name), 60*time.Second, nil); err != nil {
				return err
			}
			return s.SystemCtl.Start(s.MachineToUnit(req.Name), nil)
		}

		return s.SystemCtl.Restart(s.MachineToUnit(req.Name), nil)
	})

	return new(empty.Empty), err
}

func (s *ServiceServer) Reset(ctx context.Context, req *pb.RestartMachineRequest) (*empty.Empty, error) {
	if _, err := kvmrun.GetInstanceConf(req.Name); err != nil {
		return nil, err
	}

	err := s.RunFuncTask(ctx, req.Name+"::", func(l *log.Entry) error {
		os.Remove(s.MachineDownFile(req.Name))

		s.SystemCtl.KillBySIGKILL(s.MachineToUnit(req.Name))

		if err := s.SystemCtl.StopAndWait(s.MachineToUnit(req.Name), 30*time.Second, nil); err != nil {
			log.Errorf("Failed to shutdown %s: %s", req.Name, err)
		}

		return s.SystemCtl.Start(s.MachineToUnit(req.Name), nil)
	})

	return new(empty.Empty), err
}
