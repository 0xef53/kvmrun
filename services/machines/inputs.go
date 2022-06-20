package machines

import (
	"context"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	"github.com/0xef53/kvmrun/kvmrun"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
)

func (s *ServiceServer) AttachInputDevice(ctx context.Context, req *pb.AttachInputDeviceRequest) (*empty.Empty, error) {
	d := kvmrun.InputDevice{
		Type: strings.ReplaceAll(strings.ToLower(req.Type.String()), "_", "-"),
	}

	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.AppendInputDevice(d); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) DetachInputDevice(ctx context.Context, req *pb.DetachInputDeviceRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		devtype := strings.ReplaceAll(strings.ToLower(req.Type.String()), "_", "-")

		if err := vm.C.RemoveInputDevice(devtype); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
