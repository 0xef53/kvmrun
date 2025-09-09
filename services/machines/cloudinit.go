package machines

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) CloudInitDriveAttach(ctx context.Context, req *pb.CloudInitDriveAttachRequest) (*empty.Empty, error) {
	opts := optsFromCloudInitDriveAttachRequest(req)

	err := s.ServiceServer.Machine.CloudInitDriveAttach(ctx, req.Name, opts)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) CloudInitDriveDetach(ctx context.Context, req *pb.CloudInitDriveDetachRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.CloudInitDriveDetach(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) CloudInitDriveChangeMedia(ctx context.Context, req *pb.CloudInitDriveChangeMediaRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.CloudInitDriveChangeMedia(ctx, req.Name, req.Media, req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
