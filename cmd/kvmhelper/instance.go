package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	qmp "github.com/0xef53/go-qmp"

	"github.com/0xef53/cli"
)

var cmdCreateConf = cli.Command{
	Name:      "create-conf",
	Usage:     "create a minimalistic configuration",
	ArgsUsage: "VMNAME",
	Flags: []cli.Flag{
		cli.GenericFlag{Name: "mem", Value: &IntRange{128, 128}, PlaceHolder: "128", Usage: "memory range (actual-total) in Mb (e.g. 256-512)"},
		cli.GenericFlag{Name: "cpu", Value: &IntRange{1, 1}, PlaceHolder: "1", Usage: "virtual cpu range (e.g. 1-8, where 8 is the maximum number of hotpluggable CPUs)"},
		cli.StringFlag{Name: "launcher", PlaceHolder: "FILE", Usage: "alternative run-script to launch virtual machine"},
		cli.IntFlag{Name: "cpu-quota", PlaceHolder: "PERCENT", Usage: "the quota in percent of one CPU core (e.g., 100 or 200 or 350)"},
		cli.StringFlag{Name: "cpu-model", PlaceHolder: "NAME", Usage: "the CPU model (e.g., 'Westmere,+pcid' )"},
	},
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, createConf))
	},
}

func createConf(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	mem := c.Generic("mem").(*IntRange)
	cpu := c.Generic("cpu").(*IntRange)

	req := rpccommon.NewInstanceRequest{
		Name:      vmname,
		Launcher:  c.String("launcher"),
		MemTotal:  int(mem.Max),
		MemActual: int(mem.Min),
		CPUTotal:  int(cpu.Max),
		CPUActual: int(cpu.Min),
		CPUModel:  c.String("cpu-model"),
		CPUQuota:  c.Int("cpu-quota"),
	}

	if err := client.Request("RPC.CreateConfInstance", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors

}

var cmdRemoveConf = cli.Command{
	Name:      "remove-conf",
	Usage:     "remove an existing configuration",
	ArgsUsage: "VMNAME",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, removeConf))
	},
}

func removeConf(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
	}
	if err := client.Request("RPC.RemoveConfInstance", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}

var cmdInspect = cli.Command{
	Name:      "inspect",
	Usage:     "print details of virtual machine",
	ArgsUsage: "VMNAME",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, inspect))
	},
}

func inspect(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
	}

	var resp []byte
	if err := client.Request("RPC.GetInstanceJSON", &req, &resp); err != nil {
		return append(errors, err)
	}
	fmt.Printf("%s\n", resp)

	return errors
}

var cmdGetEvents = cli.Command{
	Name:      "events",
	Usage:     "print a list of QEMU events",
	ArgsUsage: "VMNAME",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, getEvents))
	},
}

func getEvents(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
	}

	var resp []qmp.Event
	if err := client.Request("RPC.GetQemuEvents", &req, &resp); err != nil {
		return append(errors, err)
	}

	if c.GlobalBool("json") {
		if b, err := json.MarshalIndent(resp, "", "    "); err == nil {
			fmt.Println(string(b))
		} else {
			return append(errors, err)
		}
	} else {
		for _, e := range resp {
			if b, err := json.MarshalIndent(e.Data, "", "    "); err == nil {
				fmt.Printf(
					"[%s]  %s\n%s\n\n",
					time.Unix(int64(e.Timestamp.Seconds), int64(e.Timestamp.Microseconds)).Format("2006-01-02 15:04:05.000000"),
					e.Type,
					b,
				)
			} else {
				return append(errors, err)
			}
		}
	}

	return errors
}

type TimeDuration time.Duration

func (d TimeDuration) String() string {
	day := d / (24 * TimeDuration(time.Hour))
	nsec := time.Duration(d) % (24 * time.Hour)
	if day == 0 {
		return fmt.Sprintf("%s", nsec)
	} else {
		return fmt.Sprintf("%dd%s", day, nsec)
	}
}

var cmdPrintList = cli.Command{
	Name:  "list",
	Usage: "print a list of virtual machines",
	Flags: []cli.Flag{
		cli.BoolFlag{Name: "short, s", Usage: "show only virtual machine IDs without any details"},
	},
	Action: func(c *cli.Context) {
		// Unix socket client
		rpcClient, err := rpcclient.NewUnixClient("/rpc/v1")
		if err != nil {
			Error.Fatalln(err)
		}

		listFunc := listVMs
		if c.Bool("short") {
			listFunc = listVMNames
		}
		if err := listFunc(c, rpcClient); err != nil {
			Error.Fatalln(err)
		}
	},
}

func listVMs(c *cli.Context, client *rpcclient.UnixClient) error {
	req := rpccommon.BriefRequest{}

	if len(c.Args()) > 0 {
		req.Names = c.Args()
	}

	resp := []rpccommon.VMSummary{}

	if err := client.Request("RPC.GetBrief", &req, &resp); err != nil {
		return err
	}

	printBriefHeader()
	for _, s := range resp {
		printBriefLine(&s)
	}

	return nil

}

func listVMNames(c *cli.Context, client *rpcclient.UnixClient) error {
	abc := []string{}

	if err := client.Request("RPC.GetInstanceNames", nil, &abc); err != nil {
		return err
	}

	for _, n := range abc {
		fmt.Println(n)
	}

	return nil

}

func printBriefHeader() {
	fmt.Printf(
		"%*s%*s%*s%*s%*s%*s %*s\n",
		1, "Name",
		28, "PID",
		14, "Mem(MiB)",
		9, "CPUs",
		9, "%CPU",
		14, "State",
		14, "Time",
	)
}

func printBriefLine(s *rpccommon.VMSummary) {
	formatTime := func(d time.Duration) string {
		day := d / (24 * time.Duration(time.Hour))
		nsec := time.Duration(d) % (24 * time.Hour)
		if day == 0 {
			return fmt.Sprintf("%s", nsec)
		}
		return fmt.Sprintf("%dd%s", day, nsec)

	}

	f := "---"
	pid, mems, cpus, lifetime, cpuPercent := f, f, f, f, f
	state := "cfgerror"

	if !s.HasError {
		if s.Pid != 0 {
			pid = fmt.Sprintf("%d", s.Pid)
		}
		mems = fmt.Sprintf("%d/%d", s.MemActual, s.MemTotal)
		cpus = fmt.Sprintf("%d/%d", s.CPUActual, s.CPUTotal)
		if s.CPUQuota != 0 {
			cpuPercent = fmt.Sprintf("%d", s.CPUQuota)
		}
		if s.LifeTime != 0 {
			lifetime = fmt.Sprintf("%s", formatTime(s.LifeTime))
		}
		state = s.State
	}

	fmt.Printf(
		"%*s%*s%*s%*s%*s%*s %*s\n",
		1, s.Name,
		(32 - len(s.Name)), pid,
		14, mems,
		9, cpus,
		9, cpuPercent,
		14, state,
		14, lifetime,
	)
}
