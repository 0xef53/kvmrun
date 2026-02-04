package machines

import (
	"context"

	"github.com/0xef53/kvmrun/kvmrun"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"

	empty "github.com/golang/protobuf/ptypes/empty"
)

func (s *service) Create(ctx context.Context, req *pb.CreateRequest) (*pb.CreateResponse, error) {
	opts := propertiesFromMachineOpts(req.Options)

	vm, err := s.ServiceServer.Machine.Create(ctx, req.Name, opts, req.QemuRootdir)
	if err != nil {
		return nil, err
	}

	return &pb.CreateResponse{Machine: machineToProto(vm, kvmrun.StateInactive, 0)}, nil
}

func (s *service) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	vm, err := s.ServiceServer.MachineGet(req.Name, false)
	if err != nil {
		return nil, err
	}

	if err := s.ServiceServer.Machine.Delete(ctx, req.Name, req.Force); err != nil {
		return nil, err
	}

	return &pb.DeleteResponse{Machine: machineToProto(vm, kvmrun.StateInactive, 0)}, nil
}

func (s *service) FirmwareSet(ctx context.Context, req *pb.FirmwareSetRequest) (*empty.Empty, error) {
	opts := optsFromFirmwareSetRequest(req)

	err := s.ServiceServer.Machine.FirmwareSet(ctx, req.Name, opts, req.QemuRootdir)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) FirmwareRemove(ctx context.Context, req *pb.FirmwareRemoveRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.FirmwareRemove(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) ExternalKernelSet(ctx context.Context, req *pb.ExternalKernelSetRequest) (*empty.Empty, error) {
	opts := optsFromExternalKernelSetRequest(req)

	err := s.ServiceServer.Machine.ExternalKernelSet(ctx, req.Name, opts)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *service) ExternalKernelRemove(ctx context.Context, req *pb.ExternalKernelRemoveRequest) (*empty.Empty, error) {
	err := s.ServiceServer.Machine.ExternalKernelRemove(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
