package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xef53/kvmrun/client/ifupdown"
	"github.com/0xef53/kvmrun/internal/appconf"
	"github.com/0xef53/kvmrun/kvmrun"

	grpc_client "github.com/0xef53/kvmrun/client/grpcclient"

	"github.com/urfave/cli/v3"
)

func ifupdownMain() error {
	app := new(cli.Command)

	app.Name = progname
	app.Usage = "interface for management virtual networks"
	app.HideHelpCommand = true

	// Build application config
	app.Before = func(ctx context.Context, c *cli.Command) (context.Context, error) {
		appConf, err := appconf.NewClientConfig(c.String("config"))
		if err != nil {
			return nil, err
		}

		ctx = grpc_client.AppendAppConfToContext(ctx, appConf)

		return ctx, nil
	}

	app.Action = func(ctx context.Context, c *cli.Command) (err error) {
		if c.IsSet("test-second-stage-feature") {
			return nil
		}

		ifname := strings.TrimSpace(c.Args().First())

		switch progname {
		case "ifup":
			err = ifupdown.InterfaceUp(ctx, ifname, c.Bool("second-stage"))
		case "ifdown":
			err = ifupdown.InterfaceDown(ctx, ifname)
		}

		if err != nil {
			return fmt.Errorf("%s: %w", progname, err)
		}

		return nil
	}

	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "second-stage",
			Sources: cli.EnvVars("SECOND_STAGE"),
		},
		&cli.BoolFlag{
			Name:    "test-second-stage-feature",
			Sources: cli.EnvVars("TEST_SECOND_STAGE_FEATURE"),
		},
		&cli.StringFlag{
			Name:    "config",
			Usage:   "path to the configuration file",
			Sources: cli.EnvVars("KVMRUN_CONFIG"),
			Value:   filepath.Join(kvmrun.CONFDIR, "kvmrun.ini"),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		exitWithError(err)
	}

	return nil
}
