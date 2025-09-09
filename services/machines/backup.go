package machines

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"
)

func (s *service) StartDiskBackupProcess(ctx context.Context, req *pb.StartDiskBackupRequest) (*pb.StartDiskBackupResponse, error) {
	opts := optsFromStartDiskBackupRequest(req)

	tid, err := s.ServiceServer.Machine.StartDiskBackupProcess(ctx, req.Name, opts)
	if err != nil {
		return nil, err
	}

	return &pb.StartDiskBackupResponse{TaskKey: tid}, nil
}
