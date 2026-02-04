package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/0xef53/kvmrun/internal/appconf"
	"github.com/0xef53/kvmrun/kvmrun"

	grpc_client "github.com/0xef53/kvmrun/client/grpcclient"

	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"

	"github.com/urfave/cli/v3"
)

var (
	progname string

	Info, Error *log.Logger
)

func init() {
	progname = filepath.Base(os.Args[0])

	switch progname {
	case "ifup", "ifdown":
	default:
		progname = "vnetctl"
	}

	Info = log.New(os.Stdout, progname+": ", 0)
	Error = log.New(os.Stderr, progname+": error: ", 0)
}

func main() {
	//
	// ifup/ifdown mode
	//

	switch progname {
	case "ifup", "ifdown":
		if err := ifupdownMain(); err != nil {
			Error.Fatalln(err)
		}

		return
	}

	//
	// Standard mode
	//

	app := new(cli.Command)

	app.Name = "vnetctl"
	app.Usage = "interface for management virtual networks"
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
		CommandCreateConf,
		CommandUpdateConf,
		CommandRemoveConf,
		{
			Name:     "version",
			Usage:    "print the version information",
			Category: "Other",
			Action: func(_ context.Context, _ *cli.Command) error {
				fmt.Printf("v%s, (built %s)\n", "1", runtime.Version())
				return nil
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		exitWithError(err)
	}
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
