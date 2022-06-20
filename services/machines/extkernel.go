package machines

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
)

func (s *ServiceServer) SetExternalKernel(ctx context.Context, req *pb.SetExternalKernelRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.RemoveConf {
			if err := vm.C.RemoveKernelConf(); err != nil {
				return err
			}
			return vm.C.Save()
		}

		var savingRequire bool

		if req.Image != "-1" {
			if err := vm.C.SetKernelImage(req.Image); err != nil {
				return err
			}
			savingRequire = true
		}

		if req.Cmdline != "-1" {
			if err := vm.C.SetKernelCmdline(req.Cmdline); err != nil {
				return err
			}
			savingRequire = true
		}

		if req.Initrd != "-1" {
			if err := vm.C.SetKernelInitrd(req.Initrd); err != nil {
				return err
			}
			savingRequire = true
		}

		if req.Modiso != "-1" {
			if err := vm.C.SetKernelModiso(req.Modiso); err != nil {
				return err
			}
			savingRequire = true
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
