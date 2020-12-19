package main

import (
	"net/http"

	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (h *rpcHandler) InitQemuInstance(r *http.Request, args *rpccommon.QemuInitRequest, resp *struct{}) error {
	opts := VMInitTaskOpts{
		VMName:    args.Name,
		Pid:       args.Pid,
		MemActual: args.MemActual,
	}

	return h.tasks.StartInit(&opts)
}
