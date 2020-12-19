package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	"github.com/0xef53/kvmrun/pkg/pwd"
	"github.com/0xef53/kvmrun/pkg/qemu"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (h *rpcHandler) CopyConfig(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.MigrationParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	if len(data.Overrides.Disks) > 0 {
		confDisks := args.VM.C.(*kvmrun.InstanceConf).Disks
		for orig, ovrd := range data.Overrides.Disks {
			if d := confDisks.Get(orig); d != nil {
				d.Path = ovrd
			} else {
				return &kvmrun.NotConnectedError{"instance_conf", orig}
			}
		}
	}

	manifest, err := json.Marshal(args.VM)
	if err != nil {
		return err
	}

	req := rpccommon.NewManifestInstanceRequest{
		Name:     args.Name,
		Manifest: manifest,
	}

	/* TO_REMOVE
	// Run file
	if l, err := os.Readlink(filepath.Join(kvmrun.CONFDIR, args.Name, "run")); err == nil {
		req.Launcher = l
	}

	// Finish file
	if l, err := os.Readlink(filepath.Join(kvmrun.CONFDIR, args.Name, "finish")); err == nil {
		req.Finisher = l
	}
	*/

	// Some extra files that may contain additional
	// configuration such as network settings.
	if ff, err := getExtraFiles(args.Name); err == nil {
		req.ExtraFiles = ff
	} else {
		return err
	}

	return h.rpcClient.Request(data.DstServer, "RPC.CreateConfInstanceFromManifest", &req, nil)
}

func (h *rpcHandler) StartMigrationProcess(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	var data *rpccommon.MigrationParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	opts := MigrationTaskOpts{
		VMName:    args.Name,
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

	// Check if a virt.machine exists on the dst server
	var found bool

	if err := h.rpcClient.Request(opts.DstServer, "RPC.IsConfExist", &rpccommon.VMNameRequest{Name: args.Name}, &found); err != nil {
		return err
	}
	if found {
		return fmt.Errorf("Already exists on the destination server: %s", args.Name)
	}

	attachedDisks := args.VM.R.(*kvmrun.InstanceQemu).Disks

	if len(data.Disks) > 0 {
		opts.Disks = make(kvmrun.Disks, 0, len(data.Disks))
		for _, diskPath := range data.Disks {
			if d := attachedDisks.Get(diskPath); d != nil {
				opts.Disks = append(opts.Disks, *d)
			} else {
				return &kvmrun.NotConnectedError{"instance_qemu", diskPath}
			}
		}
	}

	if len(data.Overrides.Disks) > 0 {
		attachedConfDisks := args.VM.C.(*kvmrun.InstanceConf).Disks

		opts.Overrides.Disks = make(map[string]string)

		for orig, ovrd := range data.Overrides.Disks {
			if d := attachedDisks.Get(orig); d != nil {
				// "orig" could be a short name (BaseName) of disk,
				// but we need the full name. So we use d.Path for that.
				opts.Overrides.Disks[d.Path] = ovrd
				d.Path = ovrd
			} else {
				return &kvmrun.NotConnectedError{"instance_qemu", orig}
			}

			if d := attachedConfDisks.Get(orig); d != nil {
				d.Path = ovrd
			} // It's OK if "orig" is not in the configuration
		}
	}

	if b, err := json.Marshal(args.VM); err == nil {
		opts.Manifest = b
	} else {
		return err
	}

	return h.tasks.StartMigration(&opts)
}

func (h *rpcHandler) CancelMigrationProcess(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	h.tasks.Cancel("migration:" + args.Name)
	return nil
}

func (h *rpcHandler) StartIncomingInstance(r *http.Request, args *rpccommon.NewManifestInstanceRequest, port *int) error {
	var vm IncomingVM

	if err := json.Unmarshal(args.Manifest, &vm); err != nil {
		return err
	}

	vmdir := filepath.Join(kvmrun.CONFDIR, vm.Name)
	vmlogdir := filepath.Join(kvmrun.LOGDIR, vm.Name)

	if _, err := os.Stat(filepath.Join(vmdir, "config")); err == nil {
		return fmt.Errorf("Already exists: %s", vmdir)
	}

	for _, d := range []string{vmdir, vmlogdir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	if err := vm.C.Save(); err != nil {
		return err
	}

	// Extra files
	if args.ExtraFiles != nil {
		for fname, content := range args.ExtraFiles {
			if err := ioutil.WriteFile(filepath.Join(kvmrun.CONFDIR, vm.Name, fname), content, 0644); err != nil {
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

	// Enable, start and test
	if err := h.sctl.Enable(vm.Name); err != nil {
		return err
	}

	return h.sctl.StartAndTest(vm.Name, 10*time.Second, nil)
}

func (h *rpcHandler) GetMigrationStat(r *http.Request, args *rpccommon.VMNameRequest, resp *rpccommon.MigrationTaskStat) error {
	switch b, err := ioutil.ReadFile(filepath.Join(kvmrun.CONFDIR, args.Name, ".runtime/migration_stat")); {
	case err == nil:
		return json.Unmarshal(b, resp)
	case os.IsNotExist(err):
	case err != nil:
		return err
	}

	taskID := "migration:" + args.Name

	(*resp) = *(h.tasks.MigrationStat(taskID))

	if lastErr := h.tasks.Err(taskID); lastErr != nil {
		resp.Desc = lastErr.Error()
	}

	return nil
}

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
