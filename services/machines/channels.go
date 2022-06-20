package machines

import (
	"context"

	"github.com/0xef53/kvmrun/kvmrun"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

func (s *ServiceServer) AttachChannel(ctx context.Context, req *pb.AttachChannelRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		switch v := req.Channel.(type) {
		case *pb.AttachChannelRequest_Vsock:
			if req.Live && vm.R != nil {
				if err := vm.R.AppendVSockDevice(v.Vsock.ContextID); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
					return err
				}
			}

			if err := vm.C.AppendVSockDevice(v.Vsock.ContextID); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
				return err
			}
		case *pb.AttachChannelRequest_SerialPort:
			return grpc_status.Errorf(grpc_codes.Unimplemented, "method is not available")
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) DetachChannel(ctx context.Context, req *pb.DetachChannelRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		switch req.Channel.(type) {
		case *pb.DetachChannelRequest_Vsock:
			if req.Live && vm.R != nil {
				if err := vm.R.RemoveVSockDevice(); err != nil && !kvmrun.IsNotConnectedError(err) {
					return err
				}
			}

			if err := vm.C.RemoveVSockDevice(); err != nil && !kvmrun.IsNotConnectedError(err) {
				return err
			}
		case *pb.DetachChannelRequest_SerialPort:
			return grpc_status.Errorf(grpc_codes.Unimplemented, "method is not available")
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
