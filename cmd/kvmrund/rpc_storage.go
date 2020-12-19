package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	"github.com/0xef53/kvmrun/pkg/lvm"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (h *rpcHandler) AttachDisk(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.DiskParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	d, err := kvmrun.NewDisk(data.Path)
	if err != nil {
		return err
	}

	d.Driver = data.Driver
	d.IopsRd = data.IopsRd
	d.IopsWr = data.IopsWr

	if args.Live {
		if data.Index >= 0 {
			return fmt.Errorf("Unable to set disk index in relation to the running QEMU instance")
		}
		switch err := args.VM.R.AppendDisk(*d); {
		case err == nil:
		case kvmrun.IsAlreadyConnectedError(err):
			// In this case just re-set the io limits
			if err := args.VM.R.SetDiskReadIops(data.Path, data.IopsRd); err != nil {
				return err
			}
			if err := args.VM.R.SetDiskWriteIops(data.Path, data.IopsWr); err != nil {
				return err
			}
		default:
			return err
		}
	}

	addToConf := func() error {
		if data.Index >= 0 {
			return args.VM.C.InsertDisk(*d, data.Index)
		}
		return args.VM.C.AppendDisk(*d)
	}

	switch err := addToConf(); {
	case err == nil:
	case kvmrun.IsAlreadyConnectedError(err):
		// In this case just re-set the io limits
		if err := args.VM.C.SetDiskReadIops(data.Path, data.IopsRd); err != nil {
			return err
		}
		if err := args.VM.C.SetDiskWriteIops(data.Path, data.IopsWr); err != nil {
			return err
		}
	default:
		return err
	}

	return args.VM.C.Save()
}

func (h *rpcHandler) DetachDisk(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.DiskParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	// Check if there is an active job with this disk
	var diskname string

	switch be, err := kvmrun.NewDiskBackend(data.Path); err.(type) {
	case nil:
		diskname = be.BaseName()
	case *kvmrun.UnknownDiskBackendError:
		// Try args.DiskName as a short disk name
		diskname = data.Path
	default:
		return err
	}

	taskID := "disk-copying:" + diskname + "@" + args.Name
	if h.tasks.InProgress(taskID) {
		return fmt.Errorf("found an active task for %s", diskname)
	}

	// Remove
	if args.Live {
		if err := args.VM.R.RemoveDisk(data.Path); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}
	}
	if err := args.VM.C.RemoveDisk(data.Path); err != nil && !kvmrun.IsNotConnectedError(err) {
		return err
	}

	return args.VM.C.Save()
}

func (h *rpcHandler) SetDiskIops(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.DiskParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	var savingRequire bool

	// "-1" means that the parameter should not be changed

	if data.IopsRd != -1 {
		if args.Live {
			if err := args.VM.R.SetDiskReadIops(data.Path, data.IopsRd); err != nil {
				return err
			}
		}
		if err := args.VM.C.SetDiskReadIops(data.Path, data.IopsRd); err != nil {
			return err
		}
		savingRequire = true
	}

	if data.IopsWr != -1 {
		if args.Live {
			if err := args.VM.R.SetDiskWriteIops(data.Path, data.IopsWr); err != nil {
				return err
			}
		}
		if err := args.VM.C.SetDiskWriteIops(data.Path, data.IopsWr); err != nil {
			return err
		}
		savingRequire = true
	}

	if savingRequire {
		return args.VM.C.Save()
	}

	return nil
}

func (h *rpcHandler) ResizeDisk(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.DiskParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	if args.VM.R != nil {
		return args.VM.R.ResizeDisk(data.Path)
	}

	return nil
}

func (h *rpcHandler) RemoveDiskBitmap(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.DiskParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	if args.VM.R != nil {
		return args.VM.R.RemoveDiskBitmap(data.Path)
	}

	return nil
}

func (h *rpcHandler) StartDiskCopyingProcess(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	// This operation is relevant only when
	// the virtual machine is running
	if args.VM.R == nil {
		return &kvmrun.NotRunningError{args.Name}
	}

	var data *rpccommon.DiskCopyingParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	srcDisk := args.VM.R.GetDisks().Get(data.SrcName)
	if srcDisk == nil {
		return fmt.Errorf("Unknown disk: %s", data.SrcName)
	}

	if args.VM.R.GetDisks().Exists(data.TargetURI) {
		return fmt.Errorf("Unable to work with the attached disk: %s", data.TargetURI)
	}

	var srcSize uint64

	if srcDisk.IsLocal() {
		s, err := srcDisk.Backend.Size()
		if err != nil {
			return err
		}
		srcSize = s
	} else {
		srcSize = srcDisk.QemuVirtualSize
	}

	dstDisk, err := kvmrun.NewDisk(data.TargetURI)
	if err != nil {
		return err
	}

	if ok, err := dstDisk.IsAvailable(); !ok {
		return err
	}

	if dstDisk.IsLocal() {
		dstSize, err := dstDisk.Backend.Size()
		if err != nil {
			return err
		}

		if dstSize < srcSize {
			return fmt.Errorf("Size of destination disk does not match the requested size")
		}
	}

	opts := DiskCopyingTaskOpts{
		SrcDisk:     srcDisk,
		DstDisk:     dstDisk,
		SrcSize:     srcSize,
		VMName:      args.Name,
		VMUid:       args.VM.R.Uid(),
		Incremental: data.Incremental,
		ClearBitmap: data.ClearBitmap,
	}

	return h.tasks.StartDiskCopying(&opts)
}

func (h *rpcHandler) CancelDiskJobProcess(r *http.Request, args *rpccommon.DiskJobRequest, resp *struct{}) error {
	var diskname string

	switch be, err := kvmrun.NewDiskBackend(args.DiskName); err.(type) {
	case nil:
		diskname = be.BaseName()
	case *kvmrun.UnknownDiskBackendError:
		// Try args.DiskName as a short disk name
		diskname = args.DiskName
	default:
		return err
	}

	taskID := "disk-copying:" + diskname + "@" + args.VMName

	h.tasks.Cancel(taskID)

	return nil
}

func (h *rpcHandler) GetDiskJobStat(r *http.Request, args *rpccommon.DiskJobRequest, resp *rpccommon.DiskCopyingTaskStat) error {
	var diskname string

	switch be, err := kvmrun.NewDiskBackend(args.DiskName); err.(type) {
	case nil:
		diskname = be.BaseName()
	case *kvmrun.UnknownDiskBackendError:
		// Try args.DiskName as a short disk name
		diskname = args.DiskName
	default:
		return err
	}

	taskID := "disk-copying:" + diskname + "@" + args.VMName

	(*resp) = *(h.tasks.DiskCopyingStat(taskID))

	if lastErr := h.tasks.Err(taskID); lastErr != nil {
		resp.Desc = lastErr.Error()
	}

	return nil
}

func (h *rpcHandler) CheckDisks(r *http.Request, args *rpccommon.CheckDisksRequest, resp *struct{}) error {
	for devpath := range args.Disks {
		disk, err := kvmrun.NewDisk(devpath)
		if err != nil {
			return err
		}
		if ok, err := disk.IsAvailable(); !ok {
			return err
		}
		if disk.IsLocal() {
			size, err := disk.Backend.Size()
			if err != nil {
				return err
			}
			if size < args.Disks[devpath] {
				return fmt.Errorf("Insufficient space on %s", devpath)
			}
		}
	}

	return nil
}

func (h *rpcHandler) PrepareDisks(r *http.Request, args *rpccommon.CreateDisksRequest, resp *struct{}) error {
	// TODO: This is a dirty hack for lvm drives. Should be rewritten
	for devpath := range args.Disks {
		vgname := strings.Split(devpath, "/")[2]
		lvname := strings.Split(devpath, "/")[3]
		if err := lvm.CreateVolume(vgname, lvname, args.Disks[devpath]); err != nil {
			return err
		}
	}

	return nil
}

func (h *rpcHandler) PrepareDstDisks(r *http.Request, args *rpccommon.CreateDisksRequest, resp *struct{}) error {
	return h.rpcClient.Request(args.DstServer, "RPC.PrepareDisks", args, nil)
}
