package main

import (
	pb_system "github.com/0xef53/kvmrun/api/services/system/v2"
)

func (l *launcher) Stop() error {
	req := pb_system.QemuInstanceStopRequest{
		Name:            l.vmname,
		GracefulTimeout: 30,
	}

	if _, err := l.client.QemuInstanceStop(l.ctx, &req); err != nil {
		Error.Println("stop:", err)
	}

	return nil
}
