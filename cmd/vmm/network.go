package main

import (
	"fmt"
	"os"
	"strings"

	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	"github.com/0xef53/cli"
)

var cmdAttachNetif = cli.Command{
	Name:      "attach-netif",
	Usage:     "attach a new network device",
	ArgsUsage: "VMNAME IFNAME",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "driver", Value: "virtio-net-pci", Usage: "new network device driver name"},
		cli.GenericFlag{Name: "hwaddr", Value: new(HwAddr), PlaceHolder: "HWADDR", Usage: "hardware address of a network interface"},
		cli.StringFlag{Name: "ifup-script", PlaceHolder: "FILE", Usage: "used to configure the network on the host side after a virtual machine starts"},
		cli.StringFlag{Name: "ifdown-script", PlaceHolder: "FILE", Usage: "used to destroy the network on the host side after a virtual machine stops"},
	},
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, attachNetif))
	},
}

func attachNetif(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &rpccommon.NetifParams{
			Ifname: c.Args().Tail()[0],
			Driver: c.String("driver"),
			HwAddr: c.Generic("hwaddr").(*HwAddr).String(),
			Ifup:   c.String("ifup-script"),
			Ifdown: c.String("ifdown-script"),
		},
	}

	if err := client.Request("RPC.AttachNetif", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}

var cmdDetachNetif = cli.Command{
	Name:      "detach-netif",
	Usage:     "detach an existing network device",
	ArgsUsage: "VMNAME IFNAME",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, detachNetif))
	},
}

func detachNetif(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &rpccommon.NetifParams{
			Ifname: c.Args().Tail()[0],
		},
	}

	if err := client.Request("RPC.DetachNetif", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}

var cmdUpdateNetif = cli.Command{
	Name:      "update-netif",
	Usage:     "update paramaters of a network device",
	ArgsUsage: "VMNAME IFNAME",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "ifup-script", Value: "-1", PlaceHolder: "FILE", Usage: "used to configure the network on the host side after a virtual machine starts"},
		cli.StringFlag{Name: "ifdown-script", Value: "-1", PlaceHolder: "FILE", Usage: "used to destroy the network on the host side after a virtual machine stops"},
	},
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, updateNetif))
	},
}

func updateNetif(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	params := rpccommon.NetifParams{
		Ifname: c.Args().Tail()[0],
		Ifup:   c.String("ifup-script"),
		Ifdown: c.String("ifdown-script"),
	}

	if params.Ifup == "-1" && params.Ifdown == "-1" {
		return errors
	}

	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &params,
	}

	if err := client.Request("RPC.UpdateNetif", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}

var cmdSetNetifLink = cli.Command{
	Name:      "set-netif-link",
	Usage:     "change the link status of a network device",
	ArgsUsage: "VMNAME IFNAME STATE",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, setNetifLink))
	},
}

func setNetifLink(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &rpccommon.NetifParams{
			Ifname: c.Args().Tail()[0],
		},
	}

	state := strings.ToLower(c.Args().Tail()[1])

	switch state {
	case "up":
		if err := client.Request("RPC.SetNetifLinkUp", &req, nil); err != nil {
			return append(errors, err)
		}
	case "down":
		if err := client.Request("RPC.SetNetifLinkDown", &req, nil); err != nil {
			return append(errors, err)
		}
	default:
		return append(errors, fmt.Errorf("Possible values for state are 'up' and 'down'"))
	}

	return errors
}
