package main

import (
	"encoding/json"
	"fmt"
	"os"

	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	"github.com/0xef53/cli"
)

var cmdSetVncPass = cli.Command{
	Name:      "set-vncpass",
	Usage:     "set new VNC password",
	ArgsUsage: "VMNAME",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "p", Usage: "secret passphrase"},
	},
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, setVncPass))
	},
}

func setVncPass(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
		Data: &rpccommon.VNCParams{
			Password: c.String("p"),
		},
	}

	var resp rpccommon.VNCRequisites

	if err := client.Request("RPC.GetVNCRequisites", &req, &resp); err != nil {
		return append(errors, err)
	}

	if c.GlobalBool("json") {
		if b, err := json.MarshalIndent(resp, "", "    "); err == nil {
			fmt.Println(string(b))
		} else {
			return append(errors, err)
		}
	} else {
		fmt.Printf("Password: %s\n", resp.Password)
		fmt.Printf("Display/Port: %d/%d\n", resp.Display, resp.Port)
		fmt.Printf("Websocket port: %d\n", resp.WSPort)
	}

	return errors
}
