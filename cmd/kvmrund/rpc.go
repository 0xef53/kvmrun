package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/0xef53/kvmrun/pkg/appconf"
	"github.com/0xef53/kvmrun/pkg/kvmrun"
	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
	"github.com/0xef53/kvmrun/pkg/systemd"

	qmp "github.com/0xef53/go-qmp/v2"
	rpc "github.com/gorilla/rpc/v2"
)

type rpcHandler struct {
	appConf   *appconf.KvmrunConfig
	rpcClient *rpcclient.TlsClient

	sctl *systemd.Manager

	mon   *QMPPool
	tasks *TaskPool
}

// ReleaseResources cleans all resources allocated for the virtual machine.
// All background tasks will try to be gracefully interrupted.
// This function should be called before the QEMU process begins to stop.
func (h *rpcHandler) ReleaseResources(r *http.Request, args *rpccommon.VMNameRequest, resp *struct{}) error {
	h.tasks.CancelAll(args.Name)
	h.mon.CloseMonitor(args.Name)
	return nil
}

func (h *rpcHandler) GetTasks(r *http.Request, args *struct{}, resp *map[string]bool) error {
	(*resp) = h.tasks.Stat()
	return nil
}

func (h *rpcHandler) getVMStatus(vm *kvmrun.VirtMachine) (string, error) {
	vmi := vm.C
	if vm.R != nil {
		vmi = vm.R
	}

	// Trying to get the special status of instance.
	// Exiting on success.
	if st, err := vmi.Status(); err == nil {
		switch st {
		case "incoming", "inmigrate", "migrated", "paused":
			return st, nil
		}
	} else {
		return "", err
	}

	unit, err := h.sctl.GetUnit(vm.Name)
	if err != nil {
		return "", fmt.Errorf("systemd dbus request failed: %s", err)
	}

	switch unit.ActiveState {
	case "active":
		return unit.SubState, nil
	case "deactivating":
		return "shutdown", nil
	case "failed":
		return "crashed", nil
	}

	return unit.ActiveState, nil
}

func (h *rpcHandler) requestPreHandler(info *rpc.RequestInfo, v interface{}) error {
	getvm := func(vmname string, mon *qmp.Monitor) (*kvmrun.VirtMachine, error) {
		vm, err := kvmrun.GetVirtMachine(vmname, mon)
		switch err.(type) {
		case nil:
		case *net.OpError:
			// If a QEMU process was terminated bypassing kvmrun
			// (for example: SIGTERM / SIGKILL / SIGINT) we will get
			// an error of type *net.OpError with code == syscall.ECONNRESET.
			// As a workaround for this case - just repeat the request with mon = nil
			return kvmrun.GetVirtMachine(vmname, nil)
		default:
			return nil, err
		}
		return vm, nil
	}

	switch v.(type) {
	case *rpccommon.InstanceRequest:
		vmname := v.(*rpccommon.InstanceRequest).Name

		mon, ok := h.mon.Get(vmname)
		if !ok {
			// This means that virt.machine is not running.
			// So turning off the "Live" flag.
			v.(*rpccommon.InstanceRequest).Live = false
		}

		// mon could be nil. It's OK
		vm, err := getvm(vmname, mon)
		if err != nil {
			return err
		}

		// Restrictions for moving virt.machines
		if st, err := h.getVMStatus(vm); err == nil {
			switch st {
			case "inmigrate", "migrated":
				switch info.Method {
				case "RPC.GetInstanceJSON", "RPC.CancelMigrationProcess", "RPC.GetMigrationStat", "RPC.RemoveConfInstance", "RPC.GetQemuEvents":
				default:
					return fmt.Errorf("Virtual machine is locked because of status: %s", st)
				}
			case "incoming":
				switch info.Method {
				case "RPC.GetInstanceJSON", "RPC.InitQemuInstance", "RPC.StartNBDServer", "RPC.StopNBDServer", "RPC.RemoveConfInstance", "RPC.GetQemuEvents":
				default:
					return fmt.Errorf("Virtual machine is locked because of status: %s", st)
				}
			}
		} else {
			return err
		}

		v.(*rpccommon.InstanceRequest).VM = vm
		if b, err := json.Marshal(v.(*rpccommon.InstanceRequest).Data); err == nil {
			v.(*rpccommon.InstanceRequest).DataRaw = json.RawMessage(b)
		} else {
			return err
		}
	}

	return nil
}
