package commands

import (
	"context"

	"github.com/0xef53/kvmrun/client"

	grpc_client "github.com/0xef53/kvmrun/client/grpcclient"

	cli "github.com/urfave/cli/v3"
)

var CommandConsole = &cli.Command{
	Name:      "console",
	Usage:     "connect to a virtual machine console",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Configuration",
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.MachineConsoleConnect)
	},
}
