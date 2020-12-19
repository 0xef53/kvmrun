package main

import (
	"fmt"

	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"

	"github.com/0xef53/cli"
)

var cmdGetServerTasks = cli.Command{
	Name:  "get-server-tasks",
	Usage: "get list of background tasks",
	Action: func(c *cli.Context) {
		// Unix socket client
		rpcClient, err := rpcclient.NewUnixClient("/rpc/v1")
		if err != nil {
			Error.Fatalln(err)
		}
		if err := getServerTasks(c, rpcClient); err != nil {
			Error.Fatalln(err)
		}

	},
}

func getServerTasks(c *cli.Context, client *rpcclient.UnixClient) error {
	resp := make(map[string]bool)

	if err := client.Request("RPC.GetTasks", nil, &resp); err != nil {
		return err
	}

	for tid, state := range resp {
		fmt.Printf("%s\t%s\n", tid, state)
	}

	return nil
}
