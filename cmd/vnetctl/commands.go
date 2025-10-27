package main

import (
	"context"

	"github.com/0xef53/kvmrun/client"
	"github.com/0xef53/kvmrun/client/flag_types"

	grpc_client "github.com/0xef53/kvmrun/client/grpcclient"

	cli "github.com/urfave/cli/v3"
)

var CommandCreateConf = &cli.Command{
	Name:      "create-conf",
	Usage:     "create a new network configuration",
	ArgsUsage: "VMNAME IFNAME",
	HideHelp:  true,
	Category:  "Configuration",
	Flags: []cli.Flag{
		&cli.GenericFlag{Name: "type", Aliases: []string{"t"}, Value: flag_types.DefaultNetworkSchemeType(), Usage: "network scheme `type`"},
		&cli.Uint32Flag{Name: "mtu", DefaultText: "not set", Usage: "MTU `size` on the network interface on the host side"},
		&cli.StringSliceFlag{Name: "ip", Usage: "IPv4 or IPv6 `address`"},
		&cli.StringFlag{Name: "gateway4", Value: "auto", Usage: "a manually defined default gateway `address` for IPv4"},
		&cli.StringFlag{Name: "gateway6", Value: "auto", Usage: "a manually defined default gateway `address` for IPv6"},
		&cli.Uint32Flag{Name: "in-limit", DefaultText: "not set", Usage: "incoming traffic threshold in `Mbits` (for routed scheme)"},
		&cli.Uint32Flag{Name: "out-limit", DefaultText: "not set", Usage: "outgoing traffic threshold in `Mbits` (for routed scheme)"},
		&cli.StringFlag{Name: "parent-interface", Usage: "the underlying device `name` (has properties varying by network scheme)"},
		&cli.Uint32Flag{Name: "vni", DefaultText: "not set", Usage: "VxLAN network `id`entifier"},
		&cli.Uint32Flag{Name: "vlan-id", DefaultText: "not set", Usage: "VLAN `id`entifier"},
		&cli.StringFlag{Name: "bridge-name", Usage: "bridge `name` to add the configuring interface"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.NetworkSchemeCreateConf)
	},
}

var CommandUpdateConf = &cli.Command{
	Name:      "update-conf",
	Usage:     "create a new network configuration",
	ArgsUsage: "VMNAME IFNAME",
	HideHelp:  true,
	Category:  "Configuration",
	Flags: []cli.Flag{
		&cli.Uint32Flag{Name: "mtu", DefaultText: "not set", Usage: "MTU `size` on the network interface on the host side"},
		&cli.StringSliceFlag{Name: "add-ip", Usage: "append new IPv4 or IPv6 `address`"},
		&cli.StringSliceFlag{Name: "del-ip", Usage: "remove an existing IPv4 or IPv6 `address`"},
		&cli.StringFlag{Name: "gateway4", Value: "auto", Usage: "a manually defined default gateway `address` for IPv4"},
		&cli.StringFlag{Name: "gateway6", Value: "auto", Usage: "a manually defined default gateway `address` for IPv6"},
		&cli.Uint32Flag{Name: "in-limit", DefaultText: "not set", Usage: "incoming traffic threshold (in `Mbits`)"},
		&cli.Uint32Flag{Name: "out-limit", DefaultText: "not set", Usage: "outgoing traffic threshold (in `Mbits`)"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.NetworkSchemeUpdateConf)
	},
}

var CommandRemoveConf = &cli.Command{
	Name:      "remove-conf",
	Usage:     "remove an existing configuration",
	ArgsUsage: "VMNAME IFNAME",
	HideHelp:  true,
	Category:  "Configuration",
	Action: func(ctx context.Context, c *cli.Command) error {
		return grpc_client.CommandGRPC(ctx, c, client.NetworkSchemeRemoveConf)
	},
}
