package network

import (
	"context"
	"fmt"

	pb "github.com/0xef53/kvmrun/api/services/network/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) CreateConf(ctx context.Context, req *pb.CreateConfRequest) (*empty.Empty, error) {
	if req.Options == nil {
		return nil, fmt.Errorf("grpc: empty network attributes")
	}

	opts := attrsFromNetworkSchemeOpts(req.Options)

	if err := s.ServiceServer.Network.CreateConf(ctx, req.Name, req.Options.Ifname, opts, req.Configure); err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) UpdateConf(ctx context.Context, req *pb.UpdateConfRequest) (*empty.Empty, error) {
	updates := setFromUpdateConfRequest(req)

	if err := s.ServiceServer.Network.UpdateConf(ctx, req.Name, req.Ifname, req.Apply, updates...); err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) DeleteConf(ctx context.Context, req *pb.DeleteConfRequest) (*empty.Empty, error) {
	if err := s.ServiceServer.Network.RemoveConf(ctx, req.Name, req.Ifname, req.Deconfigure); err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
