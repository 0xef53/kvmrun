package machines

import (
	"context"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) InputDeviceAttach(ctx context.Context, req *pb.InputDeviceAttachRequest) (*empty.Empty, error) {
	opts := optsFromInputDeviceAttachRequest(req)

	err := s.ServiceServer.Machine.InputDeviceAttach(ctx, req.Name, opts)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) InputDeviceDetach(ctx context.Context, req *pb.InputDeviceDetachRequest) (*empty.Empty, error) {
	deviceType := strings.ReplaceAll(strings.ToLower(req.Type.String()), "_", "-")

	err := s.ServiceServer.Machine.InputDeviceDetach(ctx, req.Name, deviceType)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
