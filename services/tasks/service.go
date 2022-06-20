package tasks

import (
	"context"
	"fmt"
	"os"

	pb "github.com/0xef53/kvmrun/api/services/tasks/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"
	"github.com/0xef53/kvmrun/services"

	empty "github.com/golang/protobuf/ptypes/empty"
	grpc "google.golang.org/grpc"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

var _ pb.TaskServiceServer = &ServiceServer{}

func init() {
	services.Register(&ServiceServer{})
}

type ServiceServer struct {
	*services.ServiceServer
}

func (s *ServiceServer) Init(inner *services.ServiceServer) {
	s.ServiceServer = inner
}

func (s *ServiceServer) Name() string {
	return fmt.Sprintf("%T", s)
}

func (s *ServiceServer) Register(server *grpc.Server) {
	pb.RegisterTaskServiceServer(server, s)
}

func (s *ServiceServer) Get(ctx context.Context, req *pb.GetTaskRequest) (*pb.GetTaskResponse, error) {
	info, err := s.stat(req.Key)
	if err != nil {
		return nil, err
	}

	return &pb.GetTaskResponse{Task: info}, nil
}

func (s *ServiceServer) List(ctx context.Context, req *empty.Empty) (*pb.ListTasksResponse, error) {
	keys, err := s.keys()
	if err != nil {
		return nil, err
	}

	ss := make([]*pb_types.TaskInfo, 0, len(keys))

	for _, key := range keys {
		info, err := s.stat(key)
		if err != nil {
			return nil, err
		}
		ss = append(ss, info)
	}

	return &pb.ListTasksResponse{Tasks: ss}, nil
}

func (s *ServiceServer) ListKeys(ctx context.Context, req *empty.Empty) (*pb.ListKeysResponse, error) {
	keys, err := s.keys()
	if err != nil {
		return nil, err
	}

	return &pb.ListKeysResponse{Tasks: keys}, nil
}

func (s *ServiceServer) keys() ([]string, error) {
	keys, err := getFileSystemKeys()
	if err != nil {
		return nil, err
	}

	contains := func(value string) bool {
		for _, v := range keys {
			if v == value {
				return true
			}
		}
		return false
	}

	for _, key := range s.Tasks.List() {
		if !contains(key) {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

func (s *ServiceServer) stat(key string) (*pb_types.TaskInfo, error) {
	st, err := readStatFromFile(key)
	if err != nil {
		if os.IsNotExist(err) {
			st = s.Tasks.Stat(key)
		} else {
			return nil, err
		}
	}

	if st == nil {
		return nil, grpc_status.Errorf(grpc_codes.NotFound, "task not found: %s", key)
	}

	return taskStatToProto(st), nil
}

func (s *ServiceServer) Cancel(ctx context.Context, req *pb.CancelTaskRequest) (*empty.Empty, error) {
	s.Tasks.Cancel(req.Key)

	return new(empty.Empty), nil
}
