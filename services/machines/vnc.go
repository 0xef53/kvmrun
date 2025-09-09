package machines

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"
)

func (s *service) VNCActivate(ctx context.Context, req *pb.VNCActivateRequest) (*pb.VNCActivateResponse, error) {
	requisites, err := s.ServiceServer.Machine.VNCActivate(ctx, req.Name, req.Password)
	if err != nil {
		return nil, err
	}

	return &pb.VNCActivateResponse{Requisites: vncRequisitesToProto(requisites)}, nil
}
