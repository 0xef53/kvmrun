package system

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/system/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) GetAppConf(ctx context.Context, _ *empty.Empty) (*pb.GetAppConfResponse, error) {
	appConf := s.ServiceServer.GetAppConfig(ctx)

	return &pb.GetAppConfResponse{AppConf: appConfToProto(appConf)}, nil
}
