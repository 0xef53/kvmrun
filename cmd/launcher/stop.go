package main

import (
	"time"

	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (l *Launcher) Stop() error {
	req := rpccommon.VMShutdownRequest{
		Name:    l.vmname,
		Timeout: time.Second * 30,
		Wait:    true,
	}

	if err := l.client.Request("RPC.StopQemuInstance", &req, nil); err != nil {
		Error.Println("stop:", err)
	}

	return nil
}
