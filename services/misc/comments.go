package misc

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/misc/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) GetMachineComment(ctx context.Context, req *pb.GetMachineCommentRequest) (*pb.GetMachineCommentResponse, error) {
	data, err := s.ServiceServer.Misc.GetMachineComment(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.GetMachineCommentResponse{Data: data}, nil
}

func (s *service) UpdateMachineComment(ctx context.Context, req *pb.UpdateMachineCommentRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Misc.UpdateMachineComment(ctx, req.Name, req.Data)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) RemoveMachineComment(ctx context.Context, req *pb.RemoveMachineCommentRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Misc.RemoveMachineComment(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
