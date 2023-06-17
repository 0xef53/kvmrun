package main

import (
	"fmt"
	"os"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/network/v1"
	"github.com/0xef53/kvmrun/internal/grpcclient"

	"github.com/urfave/cli/v2"
	"github.com/vishvananda/netlink"
)

func ifupdownMain() error {
	app := cli.NewApp()

	app.Name = progname
	app.Usage = "interface for management virtual networks"
	app.HideHelpCommand = true

	app.Action = func(c *cli.Context) error {
		if c.IsSet("test-second-stage-feature") {
			return nil
		}

		ifname := strings.TrimSpace(c.Args().First())

		// Unix socket client
		conn, err := grpcclient.NewConn(kvmrundSock, nil, true)
		if err != nil {
			return fmt.Errorf("grpc dial error: %s", err)
		}
		defer conn.Close()

		client := pb.NewNetworkServiceClient(conn)

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
			EnvVars: []string{"SECOND_STAGE"},
		},
		&cli.BoolFlag{
			Name:    "test-second-stage-feature",
			EnvVars: []string{"TEST_SECOND_STAGE_FEATURE"},
		},
	}

	if err := app.Run(os.Args); err != nil {
		exitWithError(err)
	}

	return nil
}

func ifup(client pb.NetworkServiceClient, ifname string, secondStage bool) error {
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

func ifdown(client pb.NetworkServiceClient, ifname string) error {
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
