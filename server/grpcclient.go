package server

import (
	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	grpcclient "github.com/0xef53/go-grpc/client"
)

func (s *Server) KvmrunGRPC(host string, fn func(*grpc_interfaces.Kvmrun) error) error {
	hostport := host + ":9393"

	conn, err := grpcclient.NewSecureConnection(hostport, s.AppConf.TLSConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	return fn(grpc_interfaces.NewKvmrunInterface(conn))
}
