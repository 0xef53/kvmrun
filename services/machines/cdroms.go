package machines

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) CdromAttach(ctx context.Context, req *pb.CdromAttachRequest) (*empty.Empty, error) {
	opts := optsFromCdromAttachRequest(req)

	err := s.ServiceServer.Machine.CdromAttach(ctx, req.Name, opts, int(req.Position), req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) CdromDetach(ctx context.Context, req *pb.CdromDetachRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.CdromDetach(ctx, req.Name, req.DeviceName, req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) CdromChangeMedia(ctx context.Context, req *pb.CdromChangeMediaRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.CdromChangeMedia(ctx, req.Name, req.DeviceName, req.DeviceMedia, req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) CdromRemoveMedia(ctx context.Context, req *pb.CdromRemoveMediaRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.CdromRemoveMedia(ctx, req.Name, req.DeviceName, req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
