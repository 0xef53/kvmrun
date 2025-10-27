package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/0xef53/kvmrun/cmd/vmm/commands"
	"github.com/0xef53/kvmrun/internal/appconf"
	"github.com/0xef53/kvmrun/kvmrun"

	grpc_client "github.com/0xef53/kvmrun/client/grpcclient"

	cli "github.com/urfave/cli/v3"
)

func main() {
	app := new(cli.Command)

	app.Name = "vmm"
	app.Usage = "CLI interface for managing virtual machines"
	app.HideHelpCommand = true

	app.EnableShellCompletion = true

	// Build application config
	app.Before = func(ctx context.Context, c *cli.Command) (context.Context, error) {
		appConf, err := appconf.NewClientConfig(c.String("config"))
		if err != nil {
			return nil, err
		}

		ctx = grpc_client.AppendAppConfToContext(ctx, appConf)

		return ctx, nil
	}

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Usage:   "path to the configuration file",
			Sources: cli.EnvVars("KVMRUN_CONFIG"),
			Value:   filepath.Join(kvmrun.CONFDIR, "kvmrun.ini"),
		},
		&cli.BoolFlag{
			Name:    "json",
			Usage:   "show output in the JSON format if possible",
			Aliases: []string{"j"},
		},
	}

	app.Commands = []*cli.Command{
		commands.CommandCreateConf,
		commands.CommandRemoveConf,
		commands.CommandPrintList,
		commands.CommandInspect,
		commands.MemoryCommands,
		commands.CPUCommands,
		commands.BootCommands,
		commands.HostDeviceCommands,
		commands.InputDeviceCommands,
		commands.CdromCommands,
		commands.DiskCommands,
		commands.NetworkCommands,
		commands.ChannelCommands,
		commands.ExternalKernelCommands,
		commands.CloudInitCommands,
		commands.VNCCommands,
		commands.CommandConsole,
		// control actions
		commands.CommandStart,
		commands.CommandStop,
		commands.CommandRestart,
		commands.CommandReset,
		// migration & backup actions
		commands.BackupCommands,
		commands.MigrationCommands,
		// other actions
		{
			Name:     "version",
			Usage:    "print the version information",
			Category: "Other",
			Action: func(_ context.Context, _ *cli.Command) error {
				fmt.Printf("v%s, (built %s)\n", kvmrun.Version, runtime.Version())
				return nil
			},
		},
		// system actions
		commands.SystemCommands,
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		exitWithError(err)
	}
}
