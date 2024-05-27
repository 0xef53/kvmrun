package hardware

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	pb "github.com/0xef53/kvmrun/api/services/hardware/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"
	"github.com/0xef53/kvmrun/internal/pci"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/services"

	grpc "google.golang.org/grpc"
)

var _ pb.HardwareServiceServer = &ServiceServer{}

func init() {
	services.Register(&ServiceServer{})
}

type ServiceServer struct {
	*services.ServiceServer
}

func (s *ServiceServer) Init(inner *services.ServiceServer) {
	s.ServiceServer = inner
}

func (s *ServiceServer) Name() string {
	return fmt.Sprintf("%T", s)
}

func (s *ServiceServer) Register(server *grpc.Server) {
	pb.RegisterHardwareServiceServer(server, s)
}

func (s *ServiceServer) ListPCI(ctx context.Context, req *pb.ListPCIRequest) (*pb.ListPCIResponse, error) {
	var devices []*pb_types.PCIDevice

	if v, err := pci.DeviceList(); err == nil {
		devices = pciDeviceListToProto(v)
	} else {
		return nil, err
	}

	// Try to check reserved devices and their holders
	vmnames, err := s.GetMachineNames()
	if err != nil {
		return nil, err
	}

	tmp := make(map[string]string)

	for _, vmname := range vmnames {
		cfg := struct {
			Devices kvmrun.HostPCIPool `json:"hostpci"`
		}{}

		b, err := os.ReadFile(filepath.Join(kvmrun.CONFDIR, vmname, "config"))
		if err != nil {
			if os.IsNotExist(err) {
				// No problem, just continue
				continue
			} else {
				return nil, err
			}
		}
		if err := json.Unmarshal(b, &cfg); err != nil {
			return nil, err
		}

		for _, hostpci := range cfg.Devices {
			addr, err := pci.AddressFromHex(hostpci.Addr)
			if err != nil {
				// No problem, just continue
				continue
			}
			tmp[addr.String()] = vmname
		}
	}

	for idx := range devices {
		if vmname, ok := tmp[devices[idx].Addr]; ok {
			devices[idx].Reserved = true
			devices[idx].Holder = vmname
		}
	}

	return &pb.ListPCIResponse{Devices: devices}, nil
}
