package main

import (
	"encoding/json"
	"fmt"

	"github.com/0xef53/kvmrun/pkg/kvmrun"

	"github.com/0xef53/cli"
)

var cmdPrintVersion = cli.Command{
	Name:  "version",
	Usage: "print the version information",
	Action: func(c *cli.Context) {
		if err := printVersion(c); err != nil {
			Error.Fatalln(err)
		}
	},
}

func printVersion(c *cli.Context) error {
	if c.GlobalBool("json") {
		v := struct {
			Version string `json:"version"`
			Digits  int    `json:"digits"`
		}{
			Version: kvmrun.VERSION.String(),
			Digits:  kvmrun.VERSION.ToInt(),
		}
		jb, err := json.MarshalIndent(v, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(jb))
	} else {
		fmt.Println("Kvmrun version:", kvmrun.VERSION)
	}

	return nil
}
