package main

import (
	"os"

	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	"github.com/0xef53/cli"
)

var cmdAttachChannel = cli.Command{
	Name:      "attach-channel",
	Usage:     "attach a new communication channel",
	ArgsUsage: "VMNAME CHANNEL_ID",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "name", PlaceHolder: "org.qemu.%s.0", Usage: "virtual port name in the guest system"},
	},
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, attachChannel))
	},
}

func attachChannel(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &rpccommon.ChannelParams{
			ID:   c.Args().Tail()[0],
			Name: c.String("name"),
		},
	}

	if err := client.Request("RPC.AttachChannel", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}

var cmdDetachChannel = cli.Command{
	Name:      "detach-channel",
	Usage:     "detach an existing communication channel",
	ArgsUsage: "VMNAME CHANNEL_ID",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, detachChannel))
	},
}

func detachChannel(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &rpccommon.ChannelParams{
			ID: c.Args().Tail()[0],
		},
	}

	if err := client.Request("RPC.DetachChannel", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}
