package main

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	qmp "github.com/0xef53/go-qmp"

	rpc "github.com/gorilla/rpc/v2"
)

func requestPreHandler(info *rpc.RequestInfo, v interface{}) error {
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

		mon, ok := QPool.Get(vmname)
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
		if st, err := vm.Status(); err == nil {
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
