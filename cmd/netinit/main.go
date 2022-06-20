package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/0xef53/kvmrun/kvmrun"
)

var (
	Info  = log.New(os.Stdout, "netinit: info: ", 0)
	Error = log.New(os.Stdout, "netinit: error: ", 0)
)

func main() {
	if len(os.Args) != 2 {
		os.Exit(2)
	}
	ifname := os.Args[1]

	cwd, err := os.Getwd()
	if err != nil {
		Error.Fatalln(err)
	}
	vmname := filepath.Base(cwd)

	config := filepath.Join(kvmrun.CHROOTDIR, vmname, "run/net", ifname)

	c, err := ioutil.ReadFile(config)
	if err != nil {
		Error.Fatalln(err)
	}

	iface := kvmrun.NetIface{}
	if err := json.Unmarshal(c, &iface); err != nil {
		Error.Fatalln(err)
	}

	if iface.Ifup == "" {
		return
	}

	cmd := exec.Command(iface.Ifup, ifname)

	cmd.Stdin = bytes.NewReader(c)
	cmd.Stderr = os.Stdout
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		Error.Fatalf("cannot initialize %s: %s\n", ifname, err)
	}

	Info.Println("successfully configured:", ifname)
}
