package main

import (
	"os"

	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	"github.com/0xef53/cli"
)

var cmdSetKernel = cli.Command{
	Name:      "set-kernel",
	Usage:     "set parameters of an external kernel",
	ArgsUsage: "VMNAME",
	Flags: []cli.Flag{
		cli.BoolFlag{Name: "remove-conf", Usage: "remove an existing configuration"},
		cli.StringFlag{Name: "image", Value: "-1", PlaceHolder: "FILE", Usage: "kernel image file name"},
		cli.StringFlag{Name: "initrd", Value: "-1", PlaceHolder: "FILE", Usage: "ramdisk image file name"},
		cli.StringFlag{Name: "cmdline", Value: "-1", PlaceHolder: "FILE", Usage: "additional kernel parameters (separated by semicolon)"},
		cli.StringFlag{Name: "modiso", Value: "-1", PlaceHolder: "FILE", Usage: "name of iso image with modules"},
	},
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, setKernel))
	},
}

func setKernel(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
		Data: &rpccommon.KernelParams{
			Image:      c.String("image"),
			Initrd:     c.String("initrd"),
			Cmdline:    c.String("cmdline"),
			Modiso:     c.String("modiso"),
			RemoveConf: c.Bool("remove-conf"),
		},
	}

	if err := client.Request("RPC.SetExternalKernel", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors

}
