package main

import (
	"fmt"
	"os"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/network/v1"
	"github.com/0xef53/kvmrun/internal/grpcclient"

	"github.com/vishvananda/netlink"
)

func ifupdownMain() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: %s IFNAME", progname)
	}

	ifname := strings.TrimSpace(os.Args[1])

	// Unix socket client
	conn, err := grpcclient.NewConn(kvmrundSock, nil, true)
	if err != nil {
		return fmt.Errorf("grpc dial error: %s", err)
	}
	defer conn.Close()

	client := pb.NewNetworkServiceClient(conn)

	switch progname {
	case "ifup":
		return ifup(client, ifname)
	case "ifdown":
		return ifdown(client, ifname)
	}

	return nil
}

func ifup(client pb.NetworkServiceClient, ifname string) error {
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

	return scheme.Configure(client)
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
