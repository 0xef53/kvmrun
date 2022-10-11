package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/0xef53/kvmrun/internal/grpcclient"
	"github.com/0xef53/kvmrun/kvmrun"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

var (
	Error = log.New(os.Stdout, "Error: ", 0)
)

func init() {
	cli.AppHelpTemplate = AppHelpTemplate
	cli.CommandHelpTemplate = CommandHelpTemplate
	cli.SubcommandHelpTemplate = SubcommandHelpTemplate
}

func main() {
	app := cli.NewApp()

	app.Name = "vmm"
	app.Usage = "CLI interface for managing virtual machines"
	app.HideHelpCommand = true

	app.EnableBashCompletion = true

	app.Flags = []cli.Flag{
		&cli.BoolFlag{Name: "json", Aliases: []string{"j"}, Usage: "show output in the JSON format if possible"},
	}

	app.Commands = []*cli.Command{
		cmdConfCreate,
		cmdConfRemove,
		cmdPrintList,
		cmdInspect,
		memoryCommands,
		cpuCommands,
		bootCommands,
		inputsCommands,
		cdromCommands,
		storageCommands,
		networkCommands,
		channelsCommands,
		extkernelCommands,
		cloudinitCommands,
		vncCommands,
		cmdConsole,
		// control actions
		cmdStart,
		cmdStop,
		cmdRestart,
		cmdReset,
		// migration & backup actions
		backupCommands,
		migrationCommands,
		// other actions
		{
			Name:     "version",
			Usage:    "print the version information",
			Category: "Other",
			Action: func(c *cli.Context) error {
				fmt.Printf("v%s, (built %s)\n", kvmrun.Version, runtime.Version())
				return nil
			},
		},
		// system actions
		systemCommands,
	}

	if err := app.Run(os.Args); err != nil {
		exitWithError(err)
	}
}

func executeGRPC(c *cli.Context, f func(context.Context, string, *cli.Context, *grpc.ClientConn) error) error {
	if c.Args().Len() < countRequiredArgs(c.Command.ArgsUsage) {
		cli.ShowCommandHelpAndExit(c, c.Command.Name, 1)
	}

	vmname := c.Args().First()

	// Unix socket client
	conn, err := grpcclient.NewConn("unix:@/run/kvmrund.sock", nil, false)
	if err != nil {
		return cli.Exit(fmt.Errorf("grpc dial error: %s", err), 1)
	}
	defer conn.Close()

	return f(context.Background(), vmname, c, conn)
}

func countRequiredArgs(s string) (c int) {
	for _, v := range strings.Fields(s) {
		if !strings.HasPrefix(v, "[") {
			c++
		}
	}
	return c
}

func exitWithError(err error) {
	var exitcode int
	var exitdesc string

	if e, ok := grpc_status.FromError(err); ok {
		switch e.Code() {
		case grpc_codes.AlreadyExists, grpc_codes.NotFound:
			exitcode = 2
		case grpc_codes.Unimplemented:
			exitcode = 3
		default:
			exitcode = 5
		}

		exitdesc = e.Message()
	} else {
		exitcode = 1
		exitdesc = err.Error()
	}

	Error.Println(exitdesc)

	os.Exit(exitcode)
}

func IsGRPCError(err error, wantCode grpc_codes.Code) bool {
	if e, ok := grpc_status.FromError(err); ok {
		if e.Code() == wantCode {
			return true
		}
	}
	return false
}
