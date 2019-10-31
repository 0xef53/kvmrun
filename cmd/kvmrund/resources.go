package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (x *RPC) SetMemLimits(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.MemLimitsParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	update := func(vmi kvmrun.Instance) error {
		if data.Total == 0 {
			data.Total = vmi.GetTotalMem()
		}
		if data.Actual > vmi.GetTotalMem() {
			if err := vmi.SetTotalMem(data.Total); err != nil && err != kvmrun.ErrNotImplemented {
				return err
			}
			if err := vmi.SetActualMem(data.Actual); err != nil {
				return err
			}
		} else {
			if err := vmi.SetActualMem(data.Actual); err != nil {
				return err
			}
			if err := vmi.SetTotalMem(data.Total); err != nil && err != kvmrun.ErrNotImplemented {
				return err
			}
		}
		return nil
	}

	if args.Live {
		if data.Total > 0 && data.Total != args.VM.R.GetTotalMem() {
			return fmt.Errorf("Unable to change the total memory while the virtual machine is running")
		}
		if err := update(args.VM.R); err != nil {
			return err
		}
	}

	if err := update(args.VM.C); err != nil {
		return err
	}

	return args.VM.C.Save()
}

func (x *RPC) SetCPUCount(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.CPUCountParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	update := func(vmi kvmrun.Instance) error {
		if data.Total == 0 {
			data.Total = vmi.GetTotalCPUs()
		}
		if data.Actual > vmi.GetTotalCPUs() {
			if err := vmi.SetTotalCPUs(data.Total); err != nil && err != kvmrun.ErrNotImplemented {
				return err
			}
			if err := vmi.SetActualCPUs(data.Actual); err != nil {
				return err
			}
		} else {
			if err := vmi.SetActualCPUs(data.Actual); err != nil {
				return err
			}
			if err := vmi.SetTotalCPUs(data.Total); err != nil && err != kvmrun.ErrNotImplemented {
				return err
			}
		}
		return nil
	}

	if args.Live {
		if data.Total > 0 && data.Total != args.VM.R.GetTotalCPUs() {
			return fmt.Errorf("Unable to change the total CPU count while the virtual machine is running")
		}

		if err := update(args.VM.R); err != nil {
			return err
		}
	}

	if err := update(args.VM.C); err != nil {
		return err
	}

	return args.VM.C.Save()
}

func (x *RPC) SetCPUQuota(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var value int

	if err := json.Unmarshal(args.DataRaw, &value); err != nil {
		return err
	}

	if args.Live {
		if err := args.VM.R.SetCPUQuota(value); err != nil {
			return err
		}
	}

	if err := args.VM.C.SetCPUQuota(value); err != nil {
		return err
	}

	return args.VM.C.Save()
}

func (x *RPC) SetCPUModel(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var model string

	if err := json.Unmarshal(args.DataRaw, &model); err != nil {
		return err
	}

	if err := args.VM.C.SetCPUModel(model); err != nil {
		return err
	}

	return args.VM.C.Save()
}
