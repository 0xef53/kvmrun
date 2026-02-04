package machines

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) DiskAttach(ctx context.Context, req *pb.DiskAttachRequest) (*empty.Empty, error) {
	opts := optsFromDiskAttachRequest(req)

	err := s.ServiceServer.Machine.DiskAttach(ctx, req.Name, opts, int(req.Position), req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) DiskDetach(ctx context.Context, req *pb.DiskDetachRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.DiskDetach(ctx, req.Name, req.DiskName, req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) DiskSetReadLimit(ctx context.Context, req *pb.DiskSetIOLimitRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.DiskSetReadLimit(ctx, req.Name, req.DiskName, int(req.Iops), req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) DiskSetWriteLimit(ctx context.Context, req *pb.DiskSetIOLimitRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.DiskSetWriteLimit(ctx, req.Name, req.DiskName, int(req.Iops), req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) DiskRemoveQemuBitmap(ctx context.Context, req *pb.DiskRemoveQemuBitmapRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.DiskRemoveQemuBitmap(ctx, req.Name, req.DiskName)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) DiskResizeQemuBlockdev(ctx context.Context, req *pb.DiskResizeQemuBlockdevRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.DiskResizeQemuBlockdev(ctx, req.Name, req.DiskName)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
