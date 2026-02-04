package tasks

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/tasks/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"

	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	stats, err := s.ServiceServer.TaskGetStats(req.Key)
	if err != nil {
		return nil, err
	}

	if len(stats) == 0 {
		return nil, grpc_status.Errorf(grpc_codes.NotFound, "task not found: %s", req.Key)
	}

	return &pb.GetResponse{Task: taskStatToProto(stats[0])}, nil
}

func (s *service) List(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	stats, err := s.ServiceServer.TaskGetStats(req.Keys...)
	if err != nil {
		return nil, err
	}

	protos := make([]*pb_types.TaskInfo, 0, len(stats))

	for _, st := range stats {
		protos = append(protos, taskStatToProto(st))
	}

	return &pb.ListResponse{Tasks: protos}, nil
}

func (s *service) Cancel(ctx context.Context, req *pb.CancelRequest) (*empty.Empty, error) {
	err := s.ServiceServer.TaskCancel(req.Key)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
