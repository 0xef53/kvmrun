package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	pb "github.com/0xef53/kvmrun/api/services/system/v1"
	"github.com/0xef53/kvmrun/internal/grpcclient"
	"github.com/0xef53/kvmrun/kvmrun"
)

var (
	Info  = log.New(os.Stdout, "", 0)
	Error = log.New(os.Stdout, "error: ", 0)
)

func main() {
	if len(os.Args) != 2 {
		Info.Println("Usage: launcher start|stop|cleanup")

		os.Exit(1)
	}

	if err := run(); err != nil {
		Error.Fatalln(err)
	}
}

func run() error {
	if err := os.MkdirAll(kvmrun.QMPMONDIR, 0750); err != nil {
		return err
	}

	var vmname string

	if cwd, err := os.Getwd(); err == nil {
		vmname = filepath.Base(cwd)
	} else {
		if v, ok := os.LookupEnv("VMNAME"); ok {
			vmname = v
		} else {
			return err
		}
	}

	launcher, err := newLauncher(vmname)
	if err != nil {
		return err
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
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}

	return command()
}

type launcher struct {
	ctx    context.Context
	vmname string
	client pb.SystemServiceClient
}

func newLauncher(vmname string) (*launcher, error) {
	// Unix socket client
	conn, err := grpcclient.NewConn("unix:@/run/kvmrund.sock", nil, true)
	if err != nil {
		return nil, fmt.Errorf("grpc dial error: %s", err)
	}

	return &launcher{
		ctx:    context.Background(),
		vmname: vmname,
		client: pb.NewSystemServiceClient(conn),
	}, nil
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
