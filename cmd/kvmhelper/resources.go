package main

import (
	"os"
	"strconv"

	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	"github.com/0xef53/cli"
)

var cmdSetMemory = cli.Command{
	Name:      "set-memory",
	Usage:     "change the memory allocation",
	ArgsUsage: "VMNAME MEMSIZE",
	Flags: []cli.Flag{
		cli.IntFlag{Name: "total", PlaceHolder: "MAXSIZE", Usage: "total/max memory value"},
	},
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, setMemory))
	},
}

func setMemory(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	actual, err := strconv.Atoi(c.Args().Tail()[0])
	if err != nil {
		return append(errors, err)
	}

	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &rpccommon.MemLimitsParams{
			Actual: actual,
			Total:  c.Int("total"),
		},
	}

	if err := client.Request("RPC.SetMemLimits", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}

var cmdSetCPUs = cli.Command{
	Name:      "set-vcpus",
	Usage:     "change the number of virtual CPUs",
	ArgsUsage: "VMNAME COUNT",
	Flags: []cli.Flag{
		cli.IntFlag{Name: "total", PlaceHolder: "MAXCOUNT", Usage: "total/max virtual CPU count"},
	},
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, setCPUs))
	},
}

func setCPUs(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	actual, err := strconv.Atoi(c.Args().Tail()[0])
	if err != nil {
		return append(errors, err)
	}

	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &rpccommon.CPUCountParams{
			Actual: actual,
			Total:  c.Int("total"),
		},
	}

	if err := client.Request("RPC.SetCPUCount", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}

var cmdSetCPUQuota = cli.Command{
	Name:      "set-cpu-quota",
	Usage:     "change the CPU quota limit",
	ArgsUsage: "VMNAME PERCENT",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, setCPUQuota))
	},
}

func setCPUQuota(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	value, err := strconv.Atoi(c.Args().Tail()[0])
	if err != nil {
		return append(errors, err)
	}

	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: value,
	}

	if err := client.Request("RPC.SetCPUQuota", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}

var cmdSetCPUModel = cli.Command{
	Name:      "set-cpu-model",
	Usage:     "set the CPU model type (e.g, 'Westmere,+pcid' )",
	ArgsUsage: "VMNAME MODEL[+FLAGS]",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, setCPUModel))
	},
}

func setCPUModel(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
		Data: c.Args().Tail()[0],
	}

	if err := client.Request("RPC.SetCPUModel", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}
