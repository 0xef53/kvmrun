package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/client/flag_types"
	"github.com/0xef53/kvmrun/kvmrun"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func MachineCreateConf(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.CreateRequest{
		Name: vmname,
		Options: &pb_types.MachineOpts{
			Memory: new(pb_types.MachineOpts_Memory),
			CPU: &pb_types.MachineOpts_CPU{
				Model: c.String("cpu-model"),
				Quota: uint32(c.Int("cpu-quota")),
			},
		},
	}

	if c.Value("mem") != nil {
		if v, ok := c.Value("mem").(*flag_types.IntRange); ok {
			req.Options.Memory.Actual = uint32(v.Min)
			req.Options.Memory.Total = uint32(v.Max)
		}
	}

	if c.Value("cpu") != nil {
		if v, ok := c.Value("cpu").(*flag_types.IntRange); ok {
			req.Options.CPU.Actual = uint32(v.Min)
			req.Options.CPU.Total = uint32(v.Max)
		}
	}

	if c.IsSet("firmware") {
		if image := strings.TrimSpace(c.String("firmware")); len(image) > 0 {
			req.Options.Firmware = new(pb_types.MachineOpts_Firmware)

			switch image {
			case "efi", "uefi", "ovmf":
				req.Options.Firmware.Image = image
			default:
				if p, err := filepath.Abs(image); err == nil {
					req.Options.Firmware.Image = p
				} else {
					return err
				}
			}

			if flash := strings.TrimSpace(c.String("flash-device")); len(flash) > 0 {
				if p, err := filepath.Abs(flash); err == nil {
					req.Options.Firmware.Flash = p
				} else {
					return err
				}
			}
		}
	}

	if v, ok := os.LookupEnv("QEMU_ROOTDIR"); ok {
		if v = strings.TrimSpace(v); len(v) != 0 {
			if p, err := filepath.Abs(v); err == nil {
				req.QemuRootdir = p
			} else {
				return err
			}
		}
	}

	resp, err := grpcClient.Machines().Create(ctx, &req)
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

func MachineRemoveConf(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.DeleteRequest{
		Name:  vmname,
		Force: true,
	}

	resp, err := grpcClient.Machines().Delete(ctx, &req)
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

func MachineInspect(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.GetRequest{
		Name: vmname,
	}

	resp, err := grpcClient.Machines().Get(ctx, &req)
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

func MachineListEvents(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.GetEventsRequest{
		Name: vmname,
	}

	resp, err := grpcClient.Machines().GetEvents(ctx, &req)
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

func MachineList(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.ListRequest{
		Names: c.Args().Slice(),
	}

	resp, err := grpcClient.Machines().List(ctx, &req)
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
				return nsec.String()
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
				if vm.PID != 0 {
					pid = fmt.Sprintf("%d", vm.PID)
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
				lifetime = formatTime(int64(vm.LifeTime))
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

func MachineListNames(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.ListNamesRequest{
		Names: c.Args().Slice(),
	}

	resp, err := grpcClient.Machines().ListNames(ctx, &req)
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
