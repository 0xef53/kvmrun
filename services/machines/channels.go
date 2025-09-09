package machines

import (
	"context"

	"github.com/0xef53/kvmrun/kvmrun"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) ChannelAttach(ctx context.Context, req *pb.ChannelAttachRequest) (*empty.Empty, error) {
	var err error

	switch v := req.Channel.(type) {
	case *pb.ChannelAttachRequest_Vsock:
		opts := &kvmrun.ChannelVSockProperties{
			ContextID: v.Vsock.ContextID,
		}

		err = s.ServiceServer.Machine.ChannelAttach_VSock(ctx, req.Name, opts, req.Live)
	case *pb.ChannelAttachRequest_SerialPort:
		err = s.ServiceServer.Machine.ChannelAttach_SerialPort(ctx, req.Name, req.Live)
	}

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) ChannelDetach(ctx context.Context, req *pb.ChannelDetachRequest) (*empty.Empty, error) {
	var err error

	switch req.Channel.(type) {
	case *pb.ChannelDetachRequest_Vsock:
		err = s.ServiceServer.Machine.ChannelDetach_VSock(ctx, req.Name, req.Live)
	case *pb.ChannelDetachRequest_SerialPort:
		err = s.ServiceServer.Machine.ChannelDetach_SerialPort(ctx, req.Name, req.Live)
	}

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
