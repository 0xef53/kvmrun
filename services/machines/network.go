package machines

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"
	"github.com/0xef53/kvmrun/kvmrun"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
)

func (s *ServiceServer) AttachNetIface(ctx context.Context, req *pb.AttachNetIfaceRequest) (*empty.Empty, error) {
	if len(req.HwAddr) == 0 {
		if v, err := kvmrun.GenHwAddr(); err == nil {
			req.HwAddr = v
		} else {
			return nil, err
		}
	}

	n := kvmrun.NetIface{
		Ifname: req.Ifname,
		Driver: strings.ReplaceAll(strings.ToLower(req.Driver.String()), "_", "-"),
		HwAddr: req.HwAddr,
		Queues: int(req.Queues),
		Ifup:   req.IfupScript,
		Ifdown: req.IfdownScript,
	}

	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.Live && vm.R != nil {
			if err := vm.R.AppendNetIface(n); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
				return err
			}
		}

		if err := vm.C.AppendNetIface(n); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) DetachNetIface(ctx context.Context, req *pb.DetachNetIfaceRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.Live && vm.R != nil {
			if err := vm.R.RemoveNetIface(req.Ifname); err != nil && !kvmrun.IsNotConnectedError(err) {
				return err
			}
		}

		if err := vm.C.RemoveNetIface(req.Ifname); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) SetNetIfaceLinkState(ctx context.Context, req *pb.SetNetIfaceLinkRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if vm.R == nil {
			return fmt.Errorf("not running: %s", req.Name)
		}

		var set func(string) error

		switch req.State {
		case pb_types.NetIfaceLinkState_UP:
			set = vm.R.SetNetIfaceLinkUp
		case pb_types.NetIfaceLinkState_DOWN:
			set = vm.R.SetNetIfaceLinkDown
		default:
			return fmt.Errorf("incorrect link state: %s", req.State)
		}

		return set(req.Ifname)
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) SetNetIfaceUpScript(ctx context.Context, req *pb.SetNetIfaceScriptRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.SetNetIfaceUpScript(req.Ifname, req.Path); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) SetNetIfaceDownScript(ctx context.Context, req *pb.SetNetIfaceScriptRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.SetNetIfaceDownScript(req.Ifname, req.Path); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) SetNetIfaceQueues(ctx context.Context, req *pb.SetNetIfaceQueuesRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.SetNetIfaceQueues(req.Ifname, int(req.Queues)); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
