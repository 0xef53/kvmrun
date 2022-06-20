package main

import (
	pb "github.com/0xef53/kvmrun/api/services/system/v1"
)

func (l *launcher) Stop() error {
	req := pb.StopQemuInstanceRequest{
		Name:            l.vmname,
		GracefulTimeout: 30,
	}

	if _, err := l.client.StopQemuInstance(l.ctx, &req); err != nil {
		Error.Println("stop:", err)
	}

	return nil
}
