package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
)

var (
	Info  = log.New(os.Stdout, "", 0)
	Error = log.New(os.Stdout, "error: ", 0)
)

func main() {
	if err := os.MkdirAll(kvmrun.QMPMONDIR, 0750); err != nil {
		Error.Fatalln(err)
	}

	var vmname string

	if cwd, err := os.Getwd(); err == nil {
		vmname = filepath.Base(cwd)
	} else {
		if v, ok := os.LookupEnv("VMNAME"); ok {
			vmname = v
		} else {
			Error.Fatalln(err)
		}
	}

	launcher, err := NewLauncher(vmname)
	if err != nil {
		Error.Fatalln(err)
	}

	if len(os.Args) == 1 {
		fmt.Println("Usage: launcher start|stop|cleanup")
		return
	}

	var command func() error

	switch os.Args[1] {
	case "start":
		command = launcher.Start
	case "stop":
		command = launcher.Stop
	case "cleanup":
		command = launcher.Cleanup
	default:
		Error.Fatalln("unknown command:", os.Args[1])
	}

	if err := command(); err != nil {
		Error.Fatalln(err)
	}
}

type Launcher struct {
	vmname string
	client *rpcclient.UnixClient
}

func NewLauncher(vmname string) (*Launcher, error) {
	l := Launcher{vmname: vmname}

	if c, err := rpcclient.NewUnixClient("/rpc/v1"); err == nil {
		l.client = c
	} else {
		return nil, err
	}

	return &l, nil
}

type NonFatalError struct {
	msg string
}

func (e *NonFatalError) Error() string {
	return e.msg
}

func IsNonFatalError(err error) bool {
	_, ok := err.(*NonFatalError)
	return ok
}
