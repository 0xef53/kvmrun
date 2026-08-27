package machines

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) VGADeviceSetType(ctx context.Context, req *pb.VGADeviceSetTypeRequest) (*empty.Empty, error) {
	opts := optsFromVGADeviceSetTypeRequest(req)

	err := s.ServiceServer.Machine.VgaDeviceSetType(ctx, req.Name, opts)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
