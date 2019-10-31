package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	"github.com/0xef53/kvmrun/pkg/pwd"
	"github.com/0xef53/kvmrun/pkg/qemu"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
	"github.com/0xef53/kvmrun/pkg/runsv"
)

// IncomingVM is an auxiliary structure to correctly converting
// incoming JSON to kvmrun.VirtMachine.
type IncomingVM struct {
	kvmrun.VirtMachine
}

func (vm *IncomingVM) UnmarshalJSON(data []byte) error {
	tmp := struct {
		Name string          `json:"name"`
		R    json.RawMessage `json:"run"`
		C    json.RawMessage `json:"conf"`
	}{}

	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	vmc := kvmrun.NewInstanceConf(tmp.Name)
	vmr := kvmrun.NewIncomingConf(tmp.Name)

	if err := json.Unmarshal(tmp.C, &vmc); err != nil {
		return err
	}

	if len(tmp.R) != 0 {
		if err := json.Unmarshal(tmp.R, &vmr); err != nil {
			return err
		}
	} else {
		vmr = nil
	}

	vm.Name = tmp.Name
	vm.C = vmc
	vm.R = vmr

	return nil
}

func (x *RPC) CopyConfig(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.MigrationParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	manifest, err := json.Marshal(args.VM)
	if err != nil {
		return err
	}

	req := rpccommon.NewManifestInstanceRequest{
		Name:     args.Name,
		Manifest: manifest,
	}

	// Run file
	if l, err := os.Readlink(filepath.Join(kvmrun.VMCONFDIR, args.Name, "run")); err == nil {
		req.Launcher = l
	}

	// Finish file
	if l, err := os.Readlink(filepath.Join(kvmrun.VMCONFDIR, args.Name, "finish")); err == nil {
		req.Finisher = l
	}

	// Some extra files that may contain additional
	// configuration such as network settings.
	if ff, err := getExtraFiles(args.Name); err == nil {
		req.ExtraFiles = ff
	} else {
		return err
	}

	return RPCClient.Request(data.DstServer, "RPC.CreateConfInstanceFromJSON", &req, nil)
}

func (x *RPC) StartMigrationProcess(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.MigrationParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	m, err := MPool.Get(args.Name)
	if err != nil {
		return err
	}

	opts := MigrationOpts{
		DstServer: data.DstServer,
	}

	if ips, err := net.LookupIP(data.DstServer); err == nil {
		opts.DstServerIPs = ips
	} else {
		return err
	}

	if args.VM.R == nil {
		return &rpccommon.MigrationError{&kvmrun.NotRunningError{args.Name}}
	}

	// TODO: check if a virt.machine exists on the dst server

	if b, err := json.Marshal(args.VM); err == nil {
		opts.Manifest = b
	} else {
		return err
	}

	knownDisks := args.VM.R.GetDisks()

	migrDisks := make(kvmrun.Disks, 0, len(data.Disks))

	for _, diskPath := range data.Disks {
		d := knownDisks.Get(diskPath)
		if d == nil {
			return fmt.Errorf("Unknown disk: %s", diskPath)
		}
		migrDisks = append(migrDisks, *d)
	}

	opts.Disks = migrDisks

	return m.Start(&opts)
}

func (x *RPC) CancelMigrationProcess(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	m, err := MPool.Get(args.Name)
	if err != nil {
		return err
	}

	return m.Cancel()
}

func (x *RPC) StartIncomingInstance(r *http.Request, args *rpccommon.NewManifestInstanceRequest, port *int) error {
	var vm IncomingVM

	if err := json.Unmarshal(args.Manifest, &vm); err != nil {
		return err
	}

	if err := kvmrun.CreateService(vm.Name); err != nil {
		return err
	}
	if err := vm.C.Save(); err != nil {
		return err
	}

	// Run file
	if len(args.Launcher) > 0 && args.Launcher != "/usr/lib/kvmrun/launcher" {
		launcher := filepath.Join(kvmrun.VMCONFDIR, vm.Name, "run")
		if err := os.Remove(launcher); err != nil {
			return err
		}
		if err := os.Symlink(args.Launcher, launcher); err != nil {
			return err
		}
	}

	// Finish file
	if len(args.Finisher) > 0 && args.Finisher != "/usr/lib/kvmrun/finisher" {
		finisher := filepath.Join(kvmrun.VMCONFDIR, vm.Name, "finish")
		if err := os.Remove(finisher); err != nil {
			return err
		}
		if err := os.Symlink(args.Finisher, finisher); err != nil {
			return err
		}
	}

	// Extra files
	if args.ExtraFiles != nil {
		for fname, content := range args.ExtraFiles {
			if err := ioutil.WriteFile(filepath.Join(kvmrun.VMCONFDIR, vm.Name, fname), content, 0644); err != nil {
				return err
			}
		}
	}

	if uid, err := pwd.CreateUser(vm.Name); err != nil {
		return err
	} else {
		(*port) = kvmrun.INCOMINGPORT + uid
	}

	mType, err := qemu.DefaultMachineType()
	if err != nil {
		return err
	}
	if vm.R.GetMachineType() == mType {
		vm.R.SetMachineType("")
	}

	// Make the incoming_config file
	if err := vm.R.Save(); err != nil {
		return err
	}
	// Run
	if err := runsv.EnableWaitPid(vm.Name, true, 10); err != nil {
		return err
	}
	if err := runsv.CheckState(vm.Name, 10); err != nil {
		return err
	}

	return nil
}

func (x *RPC) GetMigrationStat(r *http.Request, args *rpccommon.VMNameRequest, resp *rpccommon.MigrationStat) error {
	switch b, err := ioutil.ReadFile(filepath.Join(kvmrun.VMCONFDIR, args.Name, "supervise/migration_stat")); {
	case err == nil:
		return json.Unmarshal(b, resp)
	case os.IsNotExist(err):
	case err != nil:
		return err
	}

	if !MPool.Exists(args.Name) {
		resp.Status = "none"
		resp.Qemu = new(rpccommon.StatInfo)
		resp.Disks = make(map[string]*rpccommon.StatInfo)
		return nil
	}

	m, err := MPool.Get(args.Name)
	if err != nil {
		return err
	}

	(*resp) = *(m.Stat())

	if lastErr := m.Err(); lastErr != nil {
		resp.Desc = lastErr.Error()
	}

	return nil
}
