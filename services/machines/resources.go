package machines

import (
	"context"
	"fmt"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	"github.com/0xef53/kvmrun/kvmrun"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
)

func (s *ServiceServer) SetMemLimits(ctx context.Context, req *pb.SetMemLimitsRequest) (*empty.Empty, error) {
	set := func(vmi kvmrun.Instance) error {
		if req.Total == 0 {
			req.Total = int64(vmi.GetTotalMem())
		}
		if int(req.Actual) > vmi.GetTotalMem() {
			if err := vmi.SetTotalMem(int(req.Total)); err != nil && err != kvmrun.ErrNotImplemented {
				return err
			}
			if err := vmi.SetActualMem(int(req.Actual)); err != nil {
				return err
			}
		} else {
			if err := vmi.SetActualMem(int(req.Actual)); err != nil {
				return err
			}
			if err := vmi.SetTotalMem(int(req.Total)); err != nil && err != kvmrun.ErrNotImplemented {
				return err
			}
		}
		return nil
	}

	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.Live && vm.R != nil {
			if req.Total > 0 && int(req.Total) != vm.R.GetTotalMem() {
				return fmt.Errorf("unable to change the total memory while the virtual machine is running")
			}
			if err := set(vm.R); err != nil {
				return err
			}
		}

		if err := set(vm.C); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) SetCPULimits(ctx context.Context, req *pb.SetCPULimitsRequest) (*empty.Empty, error) {
	set := func(vmi kvmrun.Instance) error {
		if req.Total == 0 {
			req.Total = int64(vmi.GetTotalCPUs())
		}
		if int(req.Actual) > vmi.GetTotalCPUs() {
			if err := vmi.SetTotalCPUs(int(req.Total)); err != nil && err != kvmrun.ErrNotImplemented {
				return err
			}
			if err := vmi.SetActualCPUs(int(req.Actual)); err != nil {
				return err
			}
		} else {
			if err := vmi.SetActualCPUs(int(req.Actual)); err != nil {
				return err
			}
			if err := vmi.SetTotalCPUs(int(req.Total)); err != nil && err != kvmrun.ErrNotImplemented {
				return err
			}
		}
		return nil
	}

	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.Live && vm.R != nil {
			if req.Total > 0 && int(req.Total) != vm.R.GetTotalCPUs() {
				return fmt.Errorf("unable to change the total CPU count while the virtual machine is running")
			}

			if err := set(vm.R); err != nil {
				return err
			}
		}

		if err := set(vm.C); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) SetCPUSockets(ctx context.Context, req *pb.SetCPUSocketsRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.SetCPUSockets(int(req.Sockets)); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) SetCPUQuota(ctx context.Context, req *pb.SetCPUQuotaRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.Live && vm.R != nil {
			if err := vm.R.SetCPUQuota(int(req.Quota)); err != nil {
				return err
			}
		}

		if err := vm.C.SetCPUQuota(int(req.Quota)); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) SetCPUModel(ctx context.Context, req *pb.SetCPUModelRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.SetCPUModel(req.Model); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
