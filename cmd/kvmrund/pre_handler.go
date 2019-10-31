package main

import (
	"encoding/json"
	"fmt"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	rpc "github.com/gorilla/rpc/v2"
)

func requestPreHandler(info *rpc.RequestInfo, v interface{}) error {
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
		vm, err := kvmrun.GetVirtMachine(vmname, mon)
		if err != nil {
			return err
		}

		// Restrictions for moving virt.machines
		if st, err := vm.Status(); err == nil {
			switch st {
			case "inmigrate", "migrated":
				switch info.Method {
				case "RPC.GetInstanceJSON", "RPC.CancelMigrationProcess", "RPC.GetMigrationStat":
				default:
					return fmt.Errorf("Virtual machine is locked because of status: %s", st)
				}
			case "incoming":
				switch info.Method {
				case "RPC.GetInstanceJSON", "RPC.InitQemuInstance", "RPC.StartNBDServer", "RPC.StopNBDServer":
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
