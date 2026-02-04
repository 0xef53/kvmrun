package cloudinit

import (
	"fmt"

	"github.com/0xef53/kvmrun/services"

	pb "github.com/0xef53/kvmrun/api/services/cloudinit/v2"

	grpcserver "github.com/0xef53/go-grpc/server"

	grpc_runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	grpc "google.golang.org/grpc"
)

var _ = pb.CloudInitServiceServer(new(service))

func init() {
	grpcserver.Register(new(service), grpcserver.WithServiceBucket("kvmrun"))
}

type service struct {
	*services.ServiceServer
}

func (s *service) Init(inner *services.ServiceServer) {
	s.ServiceServer = inner
}

func (s *service) Name() string {
	return fmt.Sprintf("%T", s)
}

func (s *service) RegisterGRPC(server *grpc.Server) {
	pb.RegisterCloudInitServiceServer(server, s)
}

func (s *service) RegisterGW(_ *grpc_runtime.ServeMux, _ string, _ []grpc.DialOption) {}
