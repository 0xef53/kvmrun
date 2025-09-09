package main

import (
	"log"
	"os"

	"github.com/0xef53/kvmrun/internal/updater"

	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"

	"github.com/urfave/cli/v2"
)

var (
	Info, Error *log.Logger
)

func init() {
	Error = log.New(os.Stderr, "error: ", 0)

	updater.Writer = os.Stdout
}

func main() {
	app := cli.NewApp()

	app.Name = "update-kvmrun-package"
	app.Usage = "kvmrun update tool"
	app.ArgsUsage = "[OPTIONS] URL"

	app.HideHelpCommand = true

	app.EnableBashCompletion = true

	app.Action = run

	app.Flags = []cli.Flag{
		&cli.BoolFlag{Name: "install-only", Usage: "don't restart the kvmrun daemon, only install the new version of a package"},
	}

	if err := app.Run(os.Args); err != nil {
		exitWithError(err)
	}
}

func run(c *cli.Context) error {
	upd, err := updater.New(c.Args().First(), c.Bool("install-only"))
	if err != nil {
		return err
	}

	return upd.Run()
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
