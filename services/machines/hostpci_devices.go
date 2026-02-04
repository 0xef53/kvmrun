package machines

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) HostDeviceAttach(ctx context.Context, req *pb.HostDeviceAttachRequest) (*empty.Empty, error) {
	opts := optsFromHostDeviceAttachRequest(req)

	err := s.ServiceServer.Machine.HostDeviceAttach(ctx, req.Name, opts, req.StrictMode)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) HostDeviceDetach(ctx context.Context, req *pb.HostDeviceDetachRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.HostDeviceDetach(ctx, req.Name, req.PCIAddr)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) HostDeviceSetMultifunctionOption(ctx context.Context, req *pb.HostDeviceSetMultifunctionOptionRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.HostDeviceSetMultifunctionOption(ctx, req.Name, req.PCIAddr, req.Enabled)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) HostDeviceSetPrimaryGPUOption(ctx context.Context, req *pb.HostDeviceSetPrimaryGPUOptionRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.HostDeviceSetPrimaryGpuOption(ctx, req.Name, req.PCIAddr, req.Enabled)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
