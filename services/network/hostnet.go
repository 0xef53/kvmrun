package network

import (
	"context"

	"github.com/0xef53/kvmrun/server/network"

	pb "github.com/0xef53/kvmrun/api/services/network/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) Configure(ctx context.Context, req *pb.ConfigureRequest) (*empty.Empty, error) {
	stage := network.ConfifureStage_FIRST

	if req.SecondStage {
		stage = network.ConfifureStage_SECOND
	}

	err := s.ServiceServer.Network.ConfigureHostNetwork(ctx, req.Name, req.Ifname, stage)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) Deconfigure(ctx context.Context, req *pb.DeconfigureRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Network.DeconfigureHostNetwork(ctx, req.Name, req.Ifname)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) ListEndpoints(ctx context.Context, req *pb.ListEndpointsRequest) (*pb.ListEndpointsResponse, error) {
	return new(pb.ListEndpointsResponse), nil
}
