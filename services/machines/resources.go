package machines

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) MemorySetLimits(ctx context.Context, req *pb.MemorySetLimitsRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.MemorySetLimits(ctx, req.Name, int(req.Actual), int(req.Total), req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) CPUSetLimits(ctx context.Context, req *pb.CPUSetLimitsRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.CPUSetLimits(ctx, req.Name, int(req.Actual), int(req.Total), req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) CPUSetSockets(ctx context.Context, req *pb.CPUSetSocketsRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.CPUSetSockets(ctx, req.Name, int(req.Sockets))
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) CPUSetQuota(ctx context.Context, req *pb.CPUSetQuotaRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.CPUSetQuota(ctx, req.Name, int(req.Quota), req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) CPUSetModel(ctx context.Context, req *pb.CPUSetModelRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.CPUSetModel(ctx, req.Name, req.Model)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
