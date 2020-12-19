package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (l *Launcher) Cleanup() error {
	chrootDir := filepath.Join(kvmrun.CHROOTDIR, l.vmname)

	deconfigureIface := func(ifname string) error {
		iface := kvmrun.NetIface{}
		b, err := ioutil.ReadFile(filepath.Join(chrootDir, "run/net", ifname))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &iface); err != nil {
			return err
		}

		// Running the external finish script
		if iface.Ifdown != "" {
			Info.Printf("starting the external finish script: %s, %s\n", iface.Ifname, iface.Ifdown)

			cmd := exec.Command(iface.Ifdown, iface.Ifname)
			cmd.Stdin = strings.NewReader(string(b))
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("external script failed: %s: %s\n", iface.Ifname, err)
			}
		}

		return kvmrun.DelTapInterface(iface.Ifname)
	}

	if files, err := ioutil.ReadDir(filepath.Join(chrootDir, "run/net")); err == nil {
		for _, f := range files {
			if err := deconfigureIface(f.Name()); err == nil {
				Info.Println("cleanup: interface has been removed:", f.Name())
			} else {
				Error.Println("cleanup: failed to deconfigure interface:", err)
			}
		}
	} else {
		Error.Println("cleanup: failed to deconfigure network interfaces:", err)
	}

	if err := os.RemoveAll(chrootDir); err != nil {
		Error.Println("cleanup: failed to remove chroot environment:", err)
	}

	for _, ext := range []string{".qga", ".qmp0", ".qmp1", ".virtcon"} {
		os.Remove(filepath.Join(kvmrun.QMPMONDIR, l.vmname+ext))
	}

	if err := l.client.Request("RPC.ReleaseResources", &rpccommon.VMNameRequest{Name: l.vmname}, nil); err != nil {
		Error.Println("cleanup: failed to release resources:", err)
	}

	return nil
}
