package hardware

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/hardware/v2"
)

func (s *service) ListPCI(ctx context.Context, _ *pb.ListPCIRequest) (*pb.ListPCIResponse, error) {
	devices, err := s.ServiceServer.Hardware.GetPCIDeviceList()
	if err != nil {
		return nil, err
	}

	return &pb.ListPCIResponse{Devices: pciDeviceListToProto(devices)}, nil
}
