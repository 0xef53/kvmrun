package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/0xef53/kvmrun/internal/appconf"
	"github.com/0xef53/kvmrun/kvmrun"

	pb_system "github.com/0xef53/kvmrun/api/services/system/v2"

	grpcclient "github.com/0xef53/go-grpc/client"

	"github.com/sirupsen/logrus"
)

func init() {
	logger := logrus.New()

	logger.SetOutput(io.Discard)

	grpcclient.SetLogger(logrus.NewEntry(logger))
}

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
	client pb_system.SystemServiceClient
}

func newLauncher(vmname string) (*launcher, error) {
	appConf, err := appconf.NewClientConfig(filepath.Join(kvmrun.CONFDIR, "kvmrun.ini"))
	if err != nil {
		return nil, err
	}

	conn, err := grpcclient.NewSecureConnection("unix:@/run/kvmrund.sock", appConf.TLSConfig)
	if err != nil {
		return nil, err
	}

	return &launcher{
		ctx:    context.Background(),
		vmname: vmname,
		client: pb_system.NewSystemServiceClient(conn),
	}, nil
}
