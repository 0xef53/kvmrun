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
	"github.com/0xef53/kvmrun/kvmrun/backend/block"

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

func MachineInfo(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.GetRequest{
		Name: vmname,
	}

	resp, err := grpcClient.Machines().Get(ctx, &req)
	if err != nil {
		return err
	}

	appendLine := func(s string, a ...interface{}) string {
		switch len(a) {
		case 0:
			s += "\n"
		case 1:
			s += fmt.Sprintf("%*s : \n", 20, a[0])
		case 2:
			var format string
			switch a[1].(type) {
			case int, int32, int64, uint, uint32, uint64:
				format = "%*s : %d\n"
			case string:
				format = "%*s : %s\n"
			default:
				format = "%*s : %q\n"
			}
			s += fmt.Sprintf(format, 20, a[0], a[1])
		}
		return s
	}

	bootDevice := func(opts *pb_types.MachineOpts) string {
		var bootdev string = "default"
		var bootidx uint32 = ^uint32(0)

		for _, d := range opts.Cdrom {
			if d.Bootindex > 0 && d.Bootindex < bootidx {
				bootdev = d.Name
				bootidx = d.Bootindex
			}
		}

		for _, d := range opts.Storage {
			if d.Bootindex > 0 && d.Bootindex < bootidx {
				bootdev = d.Path
				bootidx = d.Bootindex
			}
		}

		return bootdev
	}

	printBrief := func(m *pb_types.Machine) {
		var opts *pb_types.MachineOpts

		if m.Runtime != nil {
			opts = m.Runtime
		} else {
			opts = m.Config
		}

		var s string

		// Header
		if m.Runtime != nil {
			s += fmt.Sprintf("* %s (state: %s, pid: %d", m.Name, m.State, m.PID)
		} else {
			s += fmt.Sprintf("* %s (state: %s", m.Name, m.State)
		}

		if opts.VsockDevice != nil {
			s += fmt.Sprintf(", cid: %d)", opts.VsockDevice.ContextID)
		} else {
			s += ")"
		}

		s = appendLine(s)

		// Machine type
		var machineType string

		if m.Runtime != nil {
			machineType = opts.MachineType
		} else {
			machineType = "default"
		}

		s = appendLine(s, "Machine type", machineType)
		s = appendLine(s)

		// Processor
		if opts.CPU != nil {
			s = appendLine(s, "Processor")
			s = appendLine(s, "Model", opts.CPU.Model)
			s = appendLine(s, "Actual", opts.CPU.Actual)
			s = appendLine(s, "Total", opts.CPU.Total)

			if c.Bool("verbose") {
				s = appendLine(s, "Sockets", opts.CPU.Sockets)
			}

			s = appendLine(s)
		}

		// Memory
		if opts.Memory != nil {
			s = appendLine(s, "Memory")
			s = appendLine(s, "Actual", fmt.Sprintf("%d MiB", opts.Memory.Actual))
			s = appendLine(s, "Total", fmt.Sprintf("%d MiB", opts.Memory.Total))
			s = appendLine(s)
		}

		// Firmware
		if opts.Firmware != nil {
			s = appendLine(s, "Firmware")
			s = appendLine(s, "Image", opts.Firmware.Image)
			s = appendLine(s, "Flash", opts.Firmware.Flash)
			s = appendLine(s)
		}

		s = appendLine(s, "Boot device", bootDevice(opts))

		if opts.CloudInitDrive != nil {
			s = appendLine(s, "CloudInit", opts.CloudInitDrive.Path)
		}

		s = appendLine(s)

		// Cdrom
		if count := len(opts.Cdrom); count > 0 {
			s = appendLine(s, "Cdroms", count)

			for _, d := range opts.Cdrom {
				s = appendLine(s, d.Name, fmt.Sprintf("%s, %s", d.Driver, d.Media))
			}

			s = appendLine(s)
		}

		// Storage
		if count := len(opts.Storage); count > 0 {
			s = appendLine(s, "Storage", count)

			for _, d := range opts.Storage {
				// ignore all errors -- it's OK in this case
				size, _ := block.GetSize64(d.Path)

				s = appendLine(s, filepath.Base(d.Path), fmt.Sprintf("%.2f GiB, %s", float64(size/(1<<30)), d.Driver))

				if c.Bool("verbose") {
					s = appendLine(s, "Path", d.Path)
					s = appendLine(s, "IopsRd", d.IopsRd)
					s = appendLine(s, "IopsWr", d.IopsWr)
					s = appendLine(s)
				}
			}

			s = appendLine(s)
		}

		// Network
		if count := len(opts.Network); count > 0 {
			s = appendLine(s, "Network", count)

			for _, nc := range opts.Network {
				s = appendLine(s, nc.Ifname, fmt.Sprintf("%s, %s (queue = %d)", nc.HwAddr, nc.Driver, nc.Queues))

				if c.Bool("verbose") {
					s = appendLine(s, "Ifup", nc.Ifup)
					s = appendLine(s, "Ifdown", nc.Ifdown)
					s = appendLine(s)
				}
			}

			s = appendLine(s)
		}

		// Input devices
		if count := len(opts.Inputs); count > 0 {
			s = appendLine(s, "Input devices", count)

			for _, d := range opts.Inputs {
				s = appendLine(s, "", d.Type)
			}

			s = appendLine(s)
		}

		// Host PCI devices
		if count := len(opts.HostPCI); count > 0 {
			s = appendLine(s, "Host devices", count)

			for _, d := range opts.HostPCI {
				s = appendLine(s, "", d.PCIAddr)
			}

			s = appendLine(s)
		}

		// External kernel
		if opts.Kernel != nil {
			s = appendLine(s, "External kernel")
			s = appendLine(s, "Image", opts.Kernel.Image)
			s = appendLine(s, "Cmdline", opts.Kernel.Cmdline)
			s = appendLine(s, "Initrd", opts.Kernel.Initrd)
			s = appendLine(s, "Modules", opts.Kernel.Modiso)
			s = appendLine(s)
		}

		fmt.Printf("%s", s)
	}

	printBrief(resp.Machine)

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
