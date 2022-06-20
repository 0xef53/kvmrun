package machines

import (
	"fmt"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	"github.com/0xef53/kvmrun/services"

	grpc "google.golang.org/grpc"
)

var _ pb.MachineServiceServer = &ServiceServer{}

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
	pb.RegisterMachineServiceServer(server, s)
}
