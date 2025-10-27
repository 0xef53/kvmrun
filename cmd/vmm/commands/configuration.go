package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"
	"github.com/0xef53/kvmrun/client/flag_types"

	grpc_client "github.com/0xef53/kvmrun/client/grpcclient"

	cli "github.com/urfave/cli/v3"
)

var CommandCreateConf = &cli.Command{
	Name:      "create-conf",
	Usage:     "create a minimalistic configuration",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Configuration",
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "mem", Value: &flag_types.IntRange{Min: 128, Max: 128}, Usage: "memory `range` (actual-total) in Mb (e.g. 256-512)"},
		&cli.GenericFlag{Name: "cpu", Value: &flag_types.IntRange{Min: 1, Max: 1}, Usage: "virtual cpu `range` (e.g. 1-8, where 8 is the maximum number of hotpluggable CPUs)"},
		&cli.IntFlag{Name: "cpu-quota", DefaultText: "not set", Usage: "the quota in `percent` of one CPU core (e.g., 100 or 200 or 350)"},
		&cli.StringFlag{Name: "cpu-model", Usage: "the CPU `model` (e.g., 'Westmere,+pcid' )"},
		&cli.StringFlag{Name: "firmware", Value: "", DefaultText: "not set", Usage: "firmware image file `file`"},
		&cli.StringFlag{Name: "flash-device", Value: "", DefaultText: "not set", Usage: "firmware flash device `file`"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.MachineCreateConf)
	},
}

var CommandRemoveConf = &cli.Command{
	Name:      "remove-conf",
	Usage:     "remove an existing configuration",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Configuration",
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.MachineRemoveConf)
	},
}

var CommandInspect = &cli.Command{
	Name:      "inspect",
	Usage:     "print a virtual machine details",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Configuration",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "events", Usage: "print a list of virtual machine events"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		fn := client.MachineInspect
		if c.Bool("events") {
			fn = client.MachineListEvents
		}
		return grpc_client.CommandGRPC(ctx, c, fn)
	},
}

var CommandPrintList = &cli.Command{
	Name:     "list",
	Usage:    "print a list of virtual machines",
	HideHelp: true,
	Category: "Configuration",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "short", Aliases: []string{"s"}, Usage: "show only names without details"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		fn := client.MachineList
		if c.Bool("short") {
			fn = client.MachineListNames
		}
		return grpc_client.CommandGRPC(ctx, c, fn)
	},
}
