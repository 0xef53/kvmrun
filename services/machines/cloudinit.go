package machines

import (
	"context"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	"github.com/0xef53/kvmrun/kvmrun"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
)

func (s *ServiceServer) AttachCloudInitDrive(ctx context.Context, req *pb.AttachCloudInitRequest) (*empty.Empty, error) {
	req.Driver = strings.TrimSpace(req.Driver)

	if len(req.Driver) == 0 {
		req.Driver = "ide-cd"
	} else {
		req.Driver = strings.ReplaceAll(strings.ToLower(req.Driver), "_", "-")
	}

	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.SetCloudInitMedia(req.Media); err != nil {
			return err
		}
		if err := vm.C.SetCloudInitDriver(req.Driver); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) DetachCloudInitDrive(ctx context.Context, req *pb.DetachCloudInitRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.RemoveCloudInitConf(); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) ChangeCloudInitDrive(ctx context.Context, req *pb.ChangeCloudInitRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.Live && vm.R != nil {
			if drive := vm.R.GetCloudInitDrive(); drive != nil {
				if err := vm.R.SetCloudInitMedia(req.Media); err != nil {
					return err
				}
			} else {
				return &kvmrun.NotConnectedError{Source: "instance_qemu", Object: "cloud-init drive"}
			}
		}

		var savingRequire bool

		if drive := vm.C.GetCloudInitDrive(); drive != nil {
			if req.Media != drive.Media {
				if err := vm.C.SetCloudInitMedia(req.Media); err != nil {
					return err
				}
				savingRequire = true
			}
		} else {
			return &kvmrun.NotConnectedError{Source: "instance_conf", Object: "cloud-init drive"}
		}

		if savingRequire {
			return vm.C.Save()
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
