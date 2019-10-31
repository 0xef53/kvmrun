package main

import (
	"encoding/json"
	"net/http"

	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (x *RPC) SetExternalKernel(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.KernelParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	if data.RemoveConf {
		if err := args.VM.C.RemoveKernelConf(); err != nil {
			return err
		}
		return args.VM.C.Save()
	}

	var savingRequire bool

	if data.Image != "-1" {
		if err := args.VM.C.SetKernelImage(data.Image); err != nil {
			return err
		}
		savingRequire = true
	}
	if data.Cmdline != "-1" {
		if err := args.VM.C.SetKernelCmdline(data.Cmdline); err != nil {
			return err
		}
		savingRequire = true
	}
	if data.Initrd != "-1" {
		if err := args.VM.C.SetKernelInitrd(data.Initrd); err != nil {
			return err
		}
		savingRequire = true
	}
	if data.Modiso != "-1" {
		if err := args.VM.C.SetKernelModiso(data.Modiso); err != nil {
			return err
		}
		savingRequire = true
	}

	if savingRequire {
		return args.VM.C.Save()
	}

	return nil
}
