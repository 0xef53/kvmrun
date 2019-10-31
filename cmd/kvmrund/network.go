package main

import (
	"encoding/json"
	"net/http"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (x *RPC) AttachNetif(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.NetifParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	if len(data.HwAddr) == 0 {
		if v, err := kvmrun.GenHwAddr(); err == nil {
			data.HwAddr = v
		} else {
			return err
		}
	}

	n := kvmrun.NetIface{
		Ifname: data.Ifname,
		Driver: data.Driver,
		HwAddr: data.HwAddr,
		Ifup:   data.Ifup,
		Ifdown: data.Ifdown,
	}

	if args.Live {
		if err := args.VM.R.AppendNetIface(n); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
			return err
		}
	}

	if err := args.VM.C.AppendNetIface(n); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
		return err
	}

	return args.VM.C.Save()
}

func (x *RPC) DetachNetif(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.NetifParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	if args.Live {
		if err := args.VM.R.RemoveNetIface(data.Ifname); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}
	}

	if err := args.VM.C.RemoveNetIface(data.Ifname); err != nil && !kvmrun.IsNotConnectedError(err) {
		return err
	}

	return args.VM.C.Save()
}

func (x *RPC) UpdateNetif(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.NetifParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	var savingRequire bool

	// "-1" means that the parameter should not be changed

	if data.Ifup != "-1" {
		if err := args.VM.C.SetNetIfaceUpScript(data.Ifname, data.Ifup); err != nil {
			return err
		}
		savingRequire = true
	}
	if data.Ifdown != "-1" {
		if err := args.VM.C.SetNetIfaceDownScript(data.Ifname, data.Ifdown); err != nil {
			return err
		}
		savingRequire = true
	}

	if savingRequire {
		return args.VM.C.Save()
	}

	return nil
}

func (x *RPC) SetNetifLinkUp(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.NetifParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	return args.VM.R.SetNetIfaceLinkUp(data.Ifname)
}

func (x *RPC) SetNetifLinkDown(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.NetifParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	return args.VM.R.SetNetIfaceLinkDown(data.Ifname)
}
