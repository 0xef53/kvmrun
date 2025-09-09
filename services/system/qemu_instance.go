package system

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/system/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) QemuInstanceRegister(ctx context.Context, req *pb.QemuInstanceRegisterRequest) (*empty.Empty, error) {
	opts := optsFromQemuInstanceRegisterRequest(req)

	_, err := s.ServiceServer.System.StartInstanceRegistration(ctx, req.Name, opts)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) QemuInstanceDeregister(ctx context.Context, req *pb.QemuInstanceDeregisterRequest) (*empty.Empty, error) {
	_, err := s.ServiceServer.System.StartInstanceDeregistration(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) QemuInstanceStop(ctx context.Context, req *pb.QemuInstanceStopRequest) (*empty.Empty, error) {
	opts := optsFromStopQemuInstanceRequest(req)

	_, err := s.ServiceServer.System.StartInstanceTermination(ctx, req.Name, opts)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
