package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"

	grpcclient "github.com/0xef53/go-grpc/client"

	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func init() {
	logger := logrus.New()

	logger.SetOutput(io.Discard)

	grpcclient.SetLogger(logrus.NewEntry(logger))
}

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

	app := cli.NewApp()

	app.Name = "vnetctl"
	app.Usage = "interface for management virtual networks"
	app.HideHelpCommand = true

	app.EnableBashCompletion = true

	app.Flags = []cli.Flag{
		&cli.BoolFlag{Name: "json", Aliases: []string{"j"}, Usage: "show output in the JSON format if possible"},
	}

	app.Commands = []*cli.Command{
		{
			Name:     "version",
			Usage:    "print the version information",
			Category: "Other",
			Action: func(c *cli.Context) error {
				fmt.Printf("v%s, (built %s)\n", "1", runtime.Version())
				return nil
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
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
