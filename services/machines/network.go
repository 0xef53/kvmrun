package machines

import (
	"context"

	"github.com/0xef53/kvmrun/kvmrun"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) NetIfaceAttach(ctx context.Context, req *pb.NetIfaceAttachRequest) (*empty.Empty, error) {
	opts := optsFromNetIfaceAttachRequest(req)

	err := s.ServiceServer.Machine.NetIfaceAttach(ctx, req.Name, opts, req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) NetIfaceDetach(ctx context.Context, req *pb.NetIfaceDetachRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.NetIfaceDetach(ctx, req.Name, req.Ifname, req.Live)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) NetIfaceSetLinkState(ctx context.Context, req *pb.NetIfaceSetLinkStateRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.NetIfaceSetLinkState(ctx, req.Name, req.Ifname, kvmrun.NetLinkState(req.State))
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) NetIfaceSetUpScript(ctx context.Context, req *pb.NetIfaceSetScriptRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.NetIfaceSetUpScript(ctx, req.Name, req.Ifname, req.Path)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) NetIfaceSetDownScript(ctx context.Context, req *pb.NetIfaceSetScriptRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.NetIfaceSetDownScript(ctx, req.Name, req.Ifname, req.Path)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) NetIfaceSetQueues(ctx context.Context, req *pb.NetIfaceSetQueuesRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.NetIfaceSetQueues(ctx, req.Name, req.Ifname, int(req.Queues))
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
