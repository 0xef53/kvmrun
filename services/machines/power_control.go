package machines

import (
	"context"
	"time"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) Start(ctx context.Context, req *pb.StartRequest) (*empty.Empty, error) {
	waitTime := time.Duration(req.WaitInterval) * time.Second

	err := s.ServiceServer.Machine.Start(ctx, req.Name, waitTime)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) Stop(ctx context.Context, req *pb.StopRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.Stop(ctx, req.Name, req.Wait, req.Force)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) Restart(ctx context.Context, req *pb.RestartRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.Restart(ctx, req.Name, req.Wait)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) Reset(ctx context.Context, req *pb.ResetRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.Reset(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
