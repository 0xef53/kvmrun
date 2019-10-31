package main

import (
	"log"
	"os"
	"strings"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	"github.com/0xef53/kvmrun/pkg/rpc/client"

	"github.com/0xef53/cli"
)

var (
	Error = log.New(os.Stderr, "Error: ", 0)
	Warn  = log.New(os.Stderr, "Warning: ", 0)
)

func main() {
	cli.AppHelpTemplate = AppHelpTemplate
	app := cli.NewApp()
	app.Name = "kvmhelper"
	app.Usage = "interface for management virtual machines"
	app.Version = kvmrun.VERSION.String()
	app.HideVersion = true
	app.HideHelp = true
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "json,j", Usage: "show output in the JSON format"},
		cli.BoolFlag{Name: "live", Usage: "affect a running virtual machine"},
	}

	app.Commands = []cli.Command{
		cmdPrintList,
		cmdCreateConf,
		cmdRemoveConf,
		cmdInspect,
		cmdSetMemory,
		cmdSetCPUs,
		cmdSetCPUQuota,
		cmdSetCPUModel,
		cmdSetVncPass,
		cmdSetKernel,
		cmdAttachDisk,
		cmdDetachDisk,
		cmdUpdateDisk,
		cmdResizeDisk,
		cmdCopyDisk,
		cmdDiskJobCancel,
		cmdDiskJobStatus,
		cmdAttachNetif,
		cmdDetachNetif,
		cmdUpdateNetif,
		cmdSetNetifLink,
		cmdAttachChannel,
		cmdDetachChannel,
		cmdConsole,
		cmdMigrate,
		cmdMigrateCancel,
		cmdMigrateStatus,
		cmdCopyConfig,
		cmdPrintVersion,
	}

	if err := app.Run(os.Args); err != nil {
		Error.Println(err)
		os.Exit(3)
	}
}

func executeRPC(c *cli.Context, f func(string, bool, *cli.Context, *rpcclient.UnixClient) []error) int {
	if len(c.Args()) < countRequiredArgs(c.Command.ArgsUsage) {
		cli.ShowCommandHelp(c, c.Command.Name)
		return 3
	}

	vmname := c.Args().First()

	// Unix socket client
	rpcClient, err := rpcclient.NewUnixClient("/rpc/v1")
	if err != nil {
		Error.Println(err)
		return 1
	}

	var liveFlag bool
	if c.GlobalBool("live") {
		liveFlag = true
	}

	var exitCode int

	if errors := f(vmname, liveFlag, c, rpcClient); len(errors) > 0 {
		for _, _err := range errors {
			switch {
			case kvmrun.IsAlreadyConnectedError(_err) || kvmrun.IsNotConnectedError(_err):
				Warn.Println(_err)
				if exitCode == 0 {
					exitCode = 2
				}
			default:
				Error.Println(_err)
				exitCode = 1
			}
		}
	}

	return exitCode
}

func countRequiredArgs(s string) (c int) {
	for _, v := range strings.Fields(s) {
		if !strings.HasPrefix(v, "[") {
			c++
		}
	}
	return c
}
