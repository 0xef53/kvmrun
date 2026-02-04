package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"

	grpc_client "github.com/0xef53/kvmrun/client/grpcclient"

	cli "github.com/urfave/cli/v3"
)

var MemoryCommands = &cli.Command{
	Name:     "mem-limits",
	Usage:    "manage memory parameters",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		CommandMemorySetParameters,
	},
}

var CommandMemorySetParameters = &cli.Command{
	Name:      "set",
	Usage:     "set various memory parameters",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.IntFlag{Name: "actual", DefaultText: "not set", Usage: "actual memory size in `MiB`"},
		&cli.IntFlag{Name: "total", DefaultText: "not set", Usage: "total/max memory size in `MiB`"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.MachineMemorySetParameters)
	},
}

var CPUCommands = &cli.Command{
	Name:     "cpu-limits",
	Usage:    "manage cpu parameters (quota, model)",
	HideHelp: true,
	Category: "Configuration",
	Commands: []*cli.Command{
		CommandCPUSetParameters,
	},
}

var CommandCPUSetParameters = &cli.Command{
	Name:      "set",
	Usage:     "set various cpu parameters",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.IntFlag{Name: "actual", DefaultText: "not set", Usage: "actual value of virtual CPU `count`"},
		&cli.IntFlag{Name: "total", DefaultText: "not set", Usage: "total/max value of virtual CPU `count`"},
		&cli.IntFlag{Name: "sockets", DefaultText: "not set", Usage: "`number` of CPU sockets"},
		&cli.IntFlag{Name: "quota", DefaultText: "not set", Usage: "`percent`age of one core"},
		&cli.StringFlag{Name: "model", Usage: "CPU model type (e.g, 'Westmere,+pcid,+ssse3' )"},
		&cli.BoolFlag{Name: "live", Usage: "affect running machine"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.MachineCPUSetParameters)
	},
}
