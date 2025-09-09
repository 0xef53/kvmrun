package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xef53/kvmrun/internal/appconf"
	"github.com/0xef53/kvmrun/kvmrun"

	pb_network "github.com/0xef53/kvmrun/api/services/network/v2"

	grpcclient "github.com/0xef53/go-grpc/client"

	"github.com/urfave/cli/v3"
	"github.com/vishvananda/netlink"
)

func ifupdownMain() error {
	app := new(cli.Command)

	app.Name = progname
	app.Usage = "interface for management virtual networks"
	app.HideHelpCommand = true

	app.Action = func(ctx context.Context, c *cli.Command) error {
		if c.IsSet("test-second-stage-feature") {
			return nil
		}

		ifname := strings.TrimSpace(c.Args().First())

		appConf, err := appconf.NewClientConfig(c.String("config"))
		if err != nil {
			return err
		}

		conn, err := grpcclient.NewSecureConnection("unix:@/run/kvmrund.sock", appConf.TLSConfig)
		if err != nil {
			return err
		}
		defer conn.Close()

		client := pb_network.NewNetworkServiceClient(conn)

		switch progname {
		case "ifup":
			return ifup(client, ifname, c.Bool("second-stage"))
		case "ifdown":
			return ifdown(client, ifname)
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

func ifup(client pb_network.NetworkServiceClient, ifname string, secondStage bool) error {
	if _, err := netlink.LinkByName(ifname); err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return fmt.Errorf("netlink: link not found: %s", ifname)
		}
		return fmt.Errorf("netlink: %s", err)
	}

	scheme, err := GetNetworkScheme(ifname, "./config_network")
	if err != nil {
		if err == errSchemeNotFound {
			Info.Println("no configuration scheme found in config_network for " + ifname)

			return nil
		}
		return err
	}

	return scheme.Configure(client, secondStage)
}

func ifdown(client pb_network.NetworkServiceClient, ifname string) error {
	/*
		TODO: need to use config_network from the virt.machine chroot
	*/

	scheme, err := GetNetworkScheme(ifname, "./config_network")
	if err != nil {
		if err == errSchemeNotFound {
			Info.Println("no configuration scheme found in config_network for " + ifname)

			return nil
		}
		return err
	}

	return scheme.Deconfigure(client)
}
