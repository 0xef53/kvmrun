package system

import (
	"context"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) ServerGracefulShutdown(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	err := s.ServiceServer.GracefulShutdown(ctx)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
