package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	"github.com/0xef53/kvmrun/internal/cloudinit"
	"github.com/0xef53/kvmrun/kvmrun"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var cloudinitCommands = &cli.Command{
	Name:     "cloudinit",
	Usage:    "manage cloud-init drives (attach, detach, build)",
	HideHelp: true,
	Category: "Configuration",
	Subcommands: []*cli.Command{
		cmdCloudInitAttach,
		cmdCloudInitDetach,
		cmdCloudInitChangeMedium,
		cmdCloudInitBuild,
	},
}

var cmdCloudInitAttach = &cli.Command{
	Name:      "attach",
	Usage:     "attach a cloud-init drive",
	ArgsUsage: "VMNAME ISO-IMAGE-FILE",
	HideHelp:  true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, attachCloudInitDrive)
	},
}

func attachCloudInitDrive(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.AttachCloudInitRequest{
		Name: vmname,
		Path: c.Args().Tail()[0],
	}

	_, err := pb.NewMachineServiceClient(conn).AttachCloudInitDrive(ctx, &req)

	return err
}

var cmdCloudInitDetach = &cli.Command{
	Name:      "detach",
	Usage:     "detach a cloud-init drive",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, detachCloudInitDrive)
	},
}

func detachCloudInitDrive(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.DetachCloudInitRequest{
		Name: vmname,
	}

	_, err := pb.NewMachineServiceClient(conn).DetachCloudInitDrive(ctx, &req)

	return err
}

var cmdCloudInitChangeMedium = &cli.Command{
	Name:      "change-medium",
	Usage:     "change the medium inserted into a guest system",
	ArgsUsage: "VMNAME ISO-IMAGE-FILE",
	HideHelp:  true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, changeCloudInitDrive)
	},
}

func changeCloudInitDrive(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := pb.ChangeCloudInitRequest{
		Name: vmname,
		Path: c.Args().Tail()[0],
	}

	_, err := pb.NewMachineServiceClient(conn).ChangeCloudInitDrive(ctx, &req)

	return err
}

var cmdCloudInitBuild = &cli.Command{
	Name:      "build",
	Usage:     "build an ISO image file",
	ArgsUsage: "VMNAME [OUTPUT-FILE]",
	HideHelp:  true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, buildCloudInitDrive)
	},
}

func buildCloudInitDrive(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	var outputFile string

	if len(c.Args().Tail()) >= 1 {
		outputFile = c.Args().Tail()[0]
	} else {
		outputFile = filepath.Join(kvmrun.CONFDIR, vmname, "config_cidata")
	}

	mainConfigFile := filepath.Join(kvmrun.CONFDIR, vmname, "config")
	netConfigFile := mainConfigFile + "_network"

	macaddrs, err := func() (map[string]string, error) {
		res := make(map[string]string)

		tmp := struct {
			Network []struct {
				Name   string `json:"ifname"`
				Hwaddr string `json:"hwaddr"`
			} `json:"network"`
		}{}

		b, err := ioutil.ReadFile(mainConfigFile)
		if err != nil {
			if os.IsNotExist(err) {
				// this is not an error and means
				// that machine has no network
				return res, nil
			}
			return nil, err
		}
		if err := json.Unmarshal(b, &tmp); err != nil {
			return nil, err
		}

		for _, netif := range tmp.Network {
			res[netif.Name] = netif.Hwaddr
		}

		return res, nil
	}()
	if err != nil {
		return fmt.Errorf("failed to build cloud-init image: %s", err)
	}

	ethernets, err := func() (map[string]cloudinit.EthernetConfig, error) {
		res := make(map[string]cloudinit.EthernetConfig)

		tmp := []struct {
			Name   string   `json:"ifname"`
			Scheme string   `json:"scheme"`
			IPs    []string `json:"ips"`
		}{}

		b, err := ioutil.ReadFile(netConfigFile)
		if err != nil {
			if os.IsNotExist(err) {
				// this is not an error and means
				// that machine has no network
				return res, nil
			}
			return nil, err
		}
		if err := json.Unmarshal(b, &tmp); err != nil {
			return nil, err
		}

		for _, netif := range tmp {
			if _, ok := macaddrs[netif.Name]; !ok || len(netif.IPs) == 0 {
				// skip unknown devices and devices without IPs
				continue
			}
			if netif.Scheme != "routed" && netif.Scheme != "bridged" {
				// skip entries with unknown schemes
				continue
			}

			var ip4addrs, ip6addrs []*net.IPNet

			for _, ipstr := range netif.IPs {
				ipnet, err := parseIPNet(ipstr)
				if err != nil {
					return nil, err
				}
				if ipnet.IP.To4() != nil {
					ip4addrs = append(ip4addrs, ipnet)
				} else {
					ip6addrs = append(ip6addrs, ipnet)
				}
			}

			cfg := cloudinit.EthernetConfig{
				Addresses: netif.IPs,
			}

			cfg.Match.MacAddress = macaddrs[netif.Name]

			if netif.Scheme == "routed" {
				// public net
				if len(ip4addrs) > 0 {
					if gw := getGatewayAddr(ip4addrs[0]); gw != nil {
						cfg.Gateway4 = gw.String()
					}
				}
				if len(ip6addrs) > 0 {
					if gw := getGatewayAddr(ip6addrs[0]); gw != nil {
						cfg.Gateway6 = gw.String()
					}
				}
				res["public"] = cfg
			} else {
				res["private"] = cfg
			}
		}

		return res, nil
	}()
	if err != nil {
		return fmt.Errorf("failed to build cloud-init image: %s", err)
	}

	data := cloudinit.Data{
		Metadata: &cloudinit.MetadataConfig{
			DSMode:     "local",
			InstanceID: "i-" + vmname,
		},
		Network: &cloudinit.NetworkConfig{
			Version:   2,
			Ethernets: ethernets,
		},
	}

	if err := cloudinit.GenImage(&data, outputFile); err != nil {
		return err
	}

	fmt.Println("Image saved to", outputFile)

	return nil
}
