package machines

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"
)

func (s *service) StartMigrationProcess(ctx context.Context, req *pb.StartMigrationRequest) (*pb.StartMigrationResponse, error) {
	opts := optsFromStartMigrationRequest(req)

	tid, err := s.ServiceServer.Machine.StartMigrationProcess(ctx, req.Name, req.DstServer, opts)
	if err != nil {
		return nil, err
	}

	return &pb.StartMigrationResponse{TaskKey: tid}, nil
}
