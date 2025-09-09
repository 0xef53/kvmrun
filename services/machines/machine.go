package machines

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"
)

func (s *service) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	vm, err := s.ServiceServer.MachineGet(req.Name, true)
	if err != nil {
		return nil, err
	}

	vmstate, err := s.ServiceServer.MachineGetStatus(vm)
	if err != nil {
		return nil, err
	}

	return &pb.GetResponse{Machine: machineToProto(vm, vmstate, 0)}, nil
}

func (s *service) GetEvents(ctx context.Context, req *pb.GetEventsRequest) (*pb.GetEventsResponse, error) {
	events, err := s.ServiceServer.MachineGetEvents(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.GetEventsResponse{Events: eventsToProto(events)}, nil
}

func (s *service) List(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	vms, err := s.ServiceServer.Machine.GetList(req.Names...)
	if err != nil {
		return nil, err
	}

	protos := make([]*pb_types.Machine, 0, len(vms))

	for _, vminfo := range vms {
		protos = append(protos, machineToProto(vminfo.M, vminfo.State, vminfo.LifeTime))
	}

	return &pb.ListResponse{Machines: protos}, nil
}

func (s *service) ListNames(ctx context.Context, req *pb.ListNamesRequest) (*pb.ListNamesResponse, error) {
	names, err := s.ServiceServer.MachineGetNames(req.Names...)
	if err != nil {
		return nil, err
	}

	return &pb.ListNamesResponse{Machines: names}, nil
}
