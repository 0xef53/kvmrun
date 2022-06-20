package network

import (
	"context"
	"fmt"

	pb "github.com/0xef53/kvmrun/api/services/network/v1"
	"github.com/0xef53/kvmrun/internal/network"
	"github.com/0xef53/kvmrun/services"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

var _ pb.NetworkServiceServer = &ServiceServer{}

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
	pb.RegisterNetworkServiceServer(server, s)
}

func (s *ServiceServer) Configure(ctx context.Context, req *pb.ConfigureRequest) (*empty.Empty, error) {
	var taskKey string

	switch v := req.Attrs.(type) {
	case *pb.ConfigureRequest_Vxlan:
		taskKey = fmt.Sprintf("network:%d:", v.Vxlan.VNI)
	default:
		taskKey = "network:unknown:"
	}

	err := s.RunFuncTask(ctx, taskKey, func(l *log.Entry) error {
		switch v := req.Attrs.(type) {
		case *pb.ConfigureRequest_Vlan:
			attrs := network.VlanDeviceAttrs{VlanID: v.Vlan.VlanID}
			return network.ConfigureVlanPort(req.LinkName, &attrs)
		case *pb.ConfigureRequest_Vxlan:
			if len(v.Vxlan.BindInterface) == 0 {
				return fmt.Errorf("empty vxlan.bind_interface value")
			}

			ips, err := GetBindAddrs(v.Vxlan.BindInterface)
			if err != nil {
				return err
			}
			if len(ips) == 0 {
				return fmt.Errorf("no IPv4 addresses found on the interface %s", v.Vxlan.BindInterface)
			}

			attrs := network.VxlanDeviceAttrs{
				VNI:   v.Vxlan.VNI,
				MTU:   v.Vxlan.MTU,
				Local: ips[0],
			}

			return network.ConfigureVxlanPort(req.LinkName, &attrs)
		case *pb.ConfigureRequest_Router:
			attrs := network.RouterDeviceAttrs{}
			return network.ConfigureRouter(req.LinkName, &attrs)
		}

		return grpc_status.Errorf(grpc_codes.Unimplemented, "unknown network scheme")
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) Deconfigure(ctx context.Context, req *pb.DeconfigureRequest) (*empty.Empty, error) {
	var taskKey string

	switch v := req.Attrs.(type) {
	case *pb.DeconfigureRequest_Vxlan:
		taskKey = fmt.Sprintf("network:%d:", v.Vxlan.VNI)
	default:
		taskKey = "network:unknown:"
	}

	err := s.RunFuncTask(ctx, taskKey, func(l *log.Entry) error {
		switch v := req.Attrs.(type) {
		case *pb.DeconfigureRequest_Vlan:
			return network.DeconfigureVlanPort(req.LinkName, v.Vlan.VlanID)
		case *pb.DeconfigureRequest_Vxlan:
			return network.DeconfigureVxlanPort(req.LinkName, v.Vxlan.VNI)
		case *pb.DeconfigureRequest_Router:
			return network.DeconfigureRouter(req.LinkName)
		}

		return grpc_status.Errorf(grpc_codes.Unimplemented, "unknown network scheme")
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) ListEndPoints(ctx context.Context, req *pb.ListEndPointsRequest) (*pb.ListEndPointsResponse, error) {
	return new(pb.ListEndPointsResponse), nil
}
