package main

import (
	"net/http"

	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	qmp "github.com/0xef53/go-qmp/v2"
)

func (h *rpcHandler) SendCont(r *http.Request, args *rpccommon.VMNameRequest, resp *struct{}) error {
	return h.mon.Run(args.Name, qmp.Command{"cont", nil}, nil)
}

func (h *rpcHandler) SendStop(r *http.Request, args *rpccommon.VMNameRequest, resp *struct{}) error {
	return h.mon.Run(args.Name, qmp.Command{"stop", nil}, nil)
}

func (h *rpcHandler) SendSystemPowerdown(r *http.Request, args *rpccommon.VMNameRequest, resp *struct{}) error {
	return h.mon.Run(args.Name, qmp.Command{"system_powerdown", nil}, nil)
}

func (h *rpcHandler) SendSystemReset(r *http.Request, args *rpccommon.VMNameRequest, resp *struct{}) error {
	return h.mon.Run(args.Name, qmp.Command{"system_reset", nil}, nil)
}
