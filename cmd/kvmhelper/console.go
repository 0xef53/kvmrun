package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	"github.com/0xef53/cli"
)

var cmdConsole = cli.Command{
	Name:      "console",
	Usage:     "connect to the virtual machine console",
	ArgsUsage: "VMNAME",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, getConsole))
	},
}

func getConsole(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
	}

	var isRunning bool
	if err := client.Request("RPC.IsInstanceRunning", &req, &isRunning); err != nil {
		return append(errors, err)
	}

	if !isRunning {
		return append(errors, &kvmrun.NotRunningError{vmname})
	}

	socatBinary := "/usr/bin/socat"
	if _, err := os.Stat(socatBinary); os.IsNotExist(err) {
		return append(errors, fmt.Errorf("Socat binary not found: %s", socatBinary))
	}

	sockPath := filepath.Join(kvmrun.QMPMONDIR, fmt.Sprint(vmname, ".virtcon"))
	socatOpts := []string{
		"/usr/bin/socat",
		"-lp", c.App.Name,
		"-L", fmt.Sprintf("%s.lock", sockPath),
		"-,raw,echo=0,escape=0x0f",
		fmt.Sprintf("UNIX-CONNECT:%s", sockPath),
	}
	fmt.Printf("Welcome to '%s' VM console\nPress Enter to start\nPress Ctrl-O to exit\n", vmname)
	if err := syscall.Exec("/usr/bin/socat", socatOpts, nil); err != nil {
		return append(errors, fmt.Errorf("Failed to run socat binary: %s", socatBinary))
	}

	return errors
}
