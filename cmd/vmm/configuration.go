package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"
	flag_types "github.com/0xef53/kvmrun/cmd/vmm/types"
	"github.com/0xef53/kvmrun/kvmrun"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var cmdConfCreate = &cli.Command{
	Name:      "create-conf",
	Usage:     "create a minimalistic configuration",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Configuration",
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "mem", Value: &flag_types.IntRange{128, 128}, Usage: "memory `range` (actual-total) in Mb (e.g. 256-512)"},
		&cli.GenericFlag{Name: "cpu", Value: &flag_types.IntRange{1, 1}, Usage: "virtual cpu `range` (e.g. 1-8, where 8 is the maximum number of hotpluggable CPUs)"},
		&cli.IntFlag{Name: "cpu-quota", DefaultText: "not set", Usage: "the quota in `percent` of one CPU core (e.g., 100 or 200 or 350)"},
		&cli.StringFlag{Name: "cpu-model", Usage: "the CPU `model` (e.g., 'Westmere,+pcid' )"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, createMachineConf)
	},
}

func createMachineConf(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	mem := c.Generic("mem").(*flag_types.IntRange)
	cpu := c.Generic("cpu").(*flag_types.IntRange)

	req := pb.CreateMachineRequest{
		Name: vmname,
		Options: &pb_types.MachineOpts{
			Memory: &pb_types.MachineOpts_Memory{
				Actual: int64(mem.Min),
				Total:  int64(mem.Max),
			},
			CPU: &pb_types.MachineOpts_CPU{
				Actual: int64(cpu.Min),
				Total:  int64(cpu.Max),
				Model:  c.String("cpu-model"),
				Quota:  int64(c.Int("cpu-quota")),
			},
		},
	}

	resp, err := pb.NewMachineServiceClient(conn).Create(ctx, &req)
	if err != nil {
		return err
	}

	if c.Bool("json") {
		b, err := json.MarshalIndent(resp, "", "    ")
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", b)
	} else {
		fmt.Println("Saved to", filepath.Join(kvmrun.CONFDIR, resp.Machine.Name))
	}

	return nil
}

var cmdConfRemove = &cli.Command{
	Name:      "remove-conf",
	Usage:     "remove an existing configuration",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Configuration",
	Action: func(c *cli.Context) error {
		return executeGRPC(c, removeMachineConf)
	},
}

func removeMachineConf(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.DeleteMachineRequest{
		Name:  vmname,
		Force: true,
	}

	resp, err := pb.NewMachineServiceClient(conn).Delete(ctx, &req)
	if err != nil {
		return err
	}

	if c.Bool("json") {
		b, err := json.MarshalIndent(resp, "", "    ")
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", b)
	} else {
		fmt.Println("Removed", filepath.Join(kvmrun.CONFDIR, resp.Machine.Name))
	}

	return nil
}

var cmdInspect = &cli.Command{
	Name:      "inspect",
	Usage:     "print a virtual machine details",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Configuration",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "events", Usage: "print a list of virtual machine events"},
	},
	Action: func(c *cli.Context) error {
		fn := inspectMachine
		if c.Bool("events") {
			fn = listMachineEvents
		}

		return executeGRPC(c, fn)
	},
}

func inspectMachine(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.GetMachineRequest{
		Name: vmname,
	}

	resp, err := pb.NewMachineServiceClient(conn).Get(ctx, &req)
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", b)

	return nil
}

func listMachineEvents(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.GetMachineRequest{
		Name: vmname,
	}

	resp, err := pb.NewMachineServiceClient(conn).GetEvents(ctx, &req)
	if err != nil {
		return err
	}

	if c.Bool("json") {
		b, err := json.MarshalIndent(resp, "", "    ")
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", b)
	} else {
		for _, e := range resp.Events {
			if b, err := json.MarshalIndent(json.RawMessage(e.Data), "", "    "); err == nil {
				fmt.Printf(
					"[%s]  %s\n%s\n\n",
					time.Unix(int64(e.Timestamp.Seconds), int64(e.Timestamp.Microseconds)).Format("2006-01-02 15:04:05.000000"),
					e.Type,
					b,
				)
			} else {
				return err
			}
		}
	}

	return nil
}

var cmdPrintList = &cli.Command{
	Name:     "list",
	Usage:    "print a list of virtual machines",
	HideHelp: true,
	Category: "Configuration",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "short", Aliases: []string{"s"}, Usage: "show only names without details"},
	},
	Action: func(c *cli.Context) error {
		fn := listMachines
		if c.Bool("short") {
			fn = listMachineNames
		}

		return executeGRPC(c, fn)
	},
}

func listMachines(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.ListMachinesRequest{
		Names: c.Args().Slice(),
	}

	resp, err := pb.NewMachineServiceClient(conn).List(ctx, &req)
	if err != nil {
		return err
	}

	printHeader := func() {
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

	printLine := func(vm *pb_types.Machine) {
		formatTime := func(d int64) string {
			day := time.Duration(d) / (24 * time.Duration(time.Hour))
			nsec := time.Duration(d) % (24 * time.Hour)
			if day == 0 {
				return fmt.Sprintf("%s", nsec)
			}
			return fmt.Sprintf("%dd%s", day, nsec)
		}

		f := "---"
		pid, mems, cpus, lifetime, cpuPercent := f, f, f, f, f

		var state string

		if vm.Config == nil {
			state = "cfgerror"
		} else {
			if vm.Runtime != nil {
				if vm.Pid != 0 {
					pid = fmt.Sprintf("%d", vm.Pid)
				}
				mems = fmt.Sprintf("%d/%d", vm.Runtime.Memory.Actual, vm.Runtime.Memory.Total)
				cpus = fmt.Sprintf("%d/%d", vm.Runtime.CPU.Actual, vm.Runtime.CPU.Total)
				if vm.Runtime.CPU.Quota != 0 {
					cpuPercent = fmt.Sprintf("%d", vm.Runtime.CPU.Quota)
				}
			} else {
				mems = fmt.Sprintf("%d/%d", vm.Config.Memory.Actual, vm.Config.Memory.Total)
				cpus = fmt.Sprintf("%d/%d", vm.Config.CPU.Actual, vm.Config.CPU.Total)
				if vm.Config.CPU.Quota != 0 {
					cpuPercent = fmt.Sprintf("%d", vm.Config.CPU.Quota)
				}
			}
			if vm.LifeTime != 0 {
				lifetime = fmt.Sprintf("%s", formatTime(vm.LifeTime))
			}

			state = strings.ToLower(vm.State.String())
		}

		fmt.Printf(
			"%*s%*s%*s%*s%*s%*s %*s\n",
			1, vm.Name,
			(32 - len(vm.Name)), pid,
			14, mems,
			9, cpus,
			9, cpuPercent,
			14, state,
			14, lifetime,
		)
	}

	if c.Bool("json") {
		b, err := json.MarshalIndent(resp, "", "    ")
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", b)
	} else {
		printHeader()
		for _, vm := range resp.Machines {
			printLine(vm)
		}
	}

	return nil
}

func listMachineNames(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.ListMachinesRequest{
		Names: c.Args().Slice(),
	}

	resp, err := pb.NewMachineServiceClient(conn).ListNames(ctx, &req)
	if err != nil {
		return err
	}

	if c.Bool("json") {
		b, err := json.MarshalIndent(resp, "", "    ")
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", b)
	} else {
		for _, name := range resp.Machines {
			fmt.Println(name)
		}
	}

	return nil
}
