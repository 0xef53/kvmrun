package network

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/network/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) Configure(ctx context.Context, req *pb.ConfigureRequest) (*empty.Empty, error) {
	opts := optsFromConfigureRequest(req)

	err := s.ServiceServer.Network.ConfigureHostNetwork(ctx, req.LinkName, opts)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) Deconfigure(ctx context.Context, req *pb.DeconfigureRequest) (*empty.Empty, error) {
	opts := optsFromDeconfigureRequest(req)

	err := s.ServiceServer.Network.DeconfigureHostNetwork(ctx, req.LinkName, opts)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) ListEndpoints(ctx context.Context, req *pb.ListEndpointsRequest) (*pb.ListEndpointsResponse, error) {
	return new(pb.ListEndpointsResponse), nil
}
