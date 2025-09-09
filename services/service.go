package services

import (
	"fmt"

	"github.com/0xef53/kvmrun/server"
	"github.com/0xef53/kvmrun/server/cloudinit"
	"github.com/0xef53/kvmrun/server/hardware"
	"github.com/0xef53/kvmrun/server/machine"
	"github.com/0xef53/kvmrun/server/network"
	"github.com/0xef53/kvmrun/server/system"

	grpcserver "github.com/0xef53/go-grpc/server"
)

type ServiceServer struct {
	*server.Server

	Machine   *machine.Server
	System    *system.Server
	Network   *network.Server
	Hardware  *hardware.Server
	CloudInit *cloudinit.Server
}

func NewServiceServer(base *server.Server) (*ServiceServer, error) {
	h := &ServiceServer{
		Server:    base,
		Machine:   &machine.Server{Server: base},
		System:    &system.Server{Server: base},
		Network:   &network.Server{Server: base},
		Hardware:  &hardware.Server{Server: base},
		CloudInit: &cloudinit.Server{Server: base},
	}

	for _, s := range grpcserver.Services("kvmrun") {
		if x, ok := s.(interface{ Init(*ServiceServer) }); ok {
			x.Init(h)
		} else {
			return nil, fmt.Errorf("invalid 'kvmrun' interface: %T", s)
		}
	}

	return h, nil
}
