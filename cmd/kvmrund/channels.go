package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (x *RPC) AttachChannel(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.ChannelParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	if len(data.Name) == 0 {
		data.Name = fmt.Sprintf("org.qemu.%s.0", data.ID)
	}

	ch := kvmrun.VirtioChannel{
		ID:   data.ID,
		Name: data.Name,
	}

	if args.Live {
		if err := args.VM.R.AppendChannel(ch); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
			return err
		}
	}

	if err := args.VM.C.AppendChannel(ch); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
		return err
	}

	return args.VM.C.Save()
}

func (x *RPC) DetachChannel(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.ChannelParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	if args.Live {
		if err := args.VM.R.RemoveChannel(data.ID); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}
	}

	if err := args.VM.C.RemoveChannel(data.ID); err != nil && !kvmrun.IsNotConnectedError(err) {
		return err
	}

	return args.VM.C.Save()
}
