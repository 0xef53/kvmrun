package system

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/system/v2"
)

func (s *service) StartIncomingMigration(ctx context.Context, req *pb.StartIncomingMigrationRequest) (*pb.StartIncomingMigrationResponse, error) {
	opts := optsFromStartIncomingMigrationRequest(req)

	requisites, err := s.ServiceServer.System.StartIncomingMigrationProcess(ctx, req.Name, opts)
	if err != nil {
		return nil, err
	}

	return &pb.StartIncomingMigrationResponse{Requisites: incomingRequisitesToProto(requisites)}, nil
}
