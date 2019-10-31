package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	cg "github.com/0xef53/kvmrun/pkg/cgroup"
	"github.com/0xef53/kvmrun/pkg/kvmrun"
	"github.com/0xef53/kvmrun/pkg/rpc/client"
	"github.com/0xef53/kvmrun/pkg/rpc/common"
)

var (
	Info  = log.New(os.Stdout, "finisher: info: ", 0)
	Error = log.New(os.Stdout, "finisher: error: ", 0)
)

func removeInterfaces(dir string) error {
	fd, err := os.Open(dir)
	switch {
	case err == nil:
	case os.IsNotExist(err):
		return nil
	default:
		return err
	}
	defer fd.Close()

	files, err := fd.Readdirnames(-1)
	if err != nil {
		return err
	}

	for _, f := range files {
		iface := kvmrun.NetIface{}
		c, err := ioutil.ReadFile(filepath.Join(dir, f))
		if err != nil {
			Error.Printf("cannot remove interface: %s: %s\n", iface.Ifname, err)
			continue
		}
		if err := json.Unmarshal(c, &iface); err != nil {
			Error.Printf("cannot remove interface: %s: %s\n", iface.Ifname, err)
			continue
		}

		// Running the external finish script
		if iface.Ifdown != "" {
			Info.Printf("starting the external finish script: %s, %s\n", iface.Ifname, iface.Ifdown)
			cmd := exec.Command(iface.Ifdown, iface.Ifname)
			cmd.Stdin = strings.NewReader(string(c))
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			if err := cmd.Run(); err != nil {
				Error.Printf("the external finish script failed: %s: %s\n", iface.Ifname, err)
				continue
			}
		}

		if err := kvmrun.DelTapInterface(iface.Ifname); err != nil {
			Error.Printf("cannot remove interface: %s: %s\n", iface.Ifname, err)
			continue
		}

		Info.Println("interface has been removed:", iface.Ifname)
	}

	return nil
}

func removeCgroups(subpath string) error {
	switch p, err := cg.GetSubsystemMountpoint("cpu"); {
	case err == nil:
		cg.DestroyCgroup(filepath.Join(p, subpath))
	case cg.IsMountpointError(err):
	default:
		return err
	}

	return nil
}

func main() {
	os.Stderr = os.Stdout

	cwd, err := os.Getwd()
	if err != nil {
		Error.Fatalln(err)
	}
	vmname := filepath.Base(cwd)

	vmChrootDir := filepath.Join(kvmrun.CHROOTDIR, vmname)

	if err := removeInterfaces(filepath.Join(vmChrootDir, "run/net")); err != nil {
		Error.Println("failed to remove network interfaces:", err)
	}

	if err := os.RemoveAll(vmChrootDir); err != nil {
		Error.Println("failed to remove chroot env:", err)
	}

	if err := removeCgroups(filepath.Join("kvmrun", vmname)); err != nil {
		Error.Println("failed to remove cgroups:", err)
	}

	if client, err := rpcclient.NewUnixClient("/rpc/v1"); err == nil {
		if err := client.Request("RPC.ReleaseResources", &rpccommon.VMNameRequest{Name: vmname}, nil); err != nil {
			Error.Printf("failed to release QMP: %s\n", err)
		}
	} else {
		Error.Println(err)
	}
}
