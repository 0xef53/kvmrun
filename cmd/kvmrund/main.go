package main

import (
	"context"
	"flag"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/0xef53/kvmrun/pkg/appconf"
	"github.com/0xef53/kvmrun/pkg/kvmrun"
	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
	rpcserver "github.com/0xef53/kvmrun/pkg/rpc/server"
	"github.com/0xef53/kvmrun/pkg/systemd"

	"github.com/gorilla/mux"
	rpc "github.com/gorilla/rpc/v2"
	jsonrpc "github.com/gorilla/rpc/v2/json2"
	log "github.com/sirupsen/logrus"
)

var DebugWriter io.Writer = ioutil.Discard

type CommandArgs struct {
	ConfigFile string
	DebugMode  bool
}

func main() {
	args := CommandArgs{ConfigFile: "/etc/kvmrun/kvmrun.ini"}

	flag.StringVar(&args.ConfigFile, "config", args.ConfigFile, "path to the config `file`")
	flag.Parse()

	if _, ok := os.LookupEnv("DEBUG"); ok {
		log.SetLevel(log.DebugLevel)
	}

	if _, ok := os.LookupEnv("WITH_TIMESTAMP"); !ok {
		log.SetFormatter(&log.TextFormatter{
			DisableColors:    true,
			DisableTimestamp: true,
		})
	}

	if err := run(&args); err != nil {
		log.Fatalln(err)
	}
}

func run(args *CommandArgs) error {
	for _, d := range []string{kvmrun.CONFDIR, kvmrun.LOGDIR} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	appConf, err := appconf.NewConfig(args.ConfigFile)
	if err != nil {
		return err
	}

	// Pool of all running virt.machines
	qmpPool := NewQMPPool(kvmrun.QMPMONDIR)

	// It's a systemd unit manager
	sctl, err := systemd.NewManager()
	if err != nil {
		return err
	}
	defer sctl.Close()

	// Try to re-create QMP pool for running virt.machines
	if n, err := monitorReConnect(qmpPool, sctl); err == nil {
		if n == 1 {
			log.Infof("Found %d running instance", n)
		} else {
			log.Infof("Found %d running instances", n)
		}
	} else {
		return err
	}

	// Global context
	ctx, cancel := context.WithCancel(context.Background())

	// Secure RPC client is used to communicate with remote Kvmrun servers
	rpcClient, err := rpcclient.NewTlsClient("/rpc/v1", appConf.Common.ClientCrt, appConf.Common.ClientKey)
	if err != nil {
		return err
	}

	// Signal handler
	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		log.Infof("Signal received: %s", <-sigc)
		cancel() // this prevents any new tasks
		// TODO: take background tasks into account (wg?)
	}()

	rpch := rpcHandler{
		appConf:   appConf,
		rpcClient: rpcClient,
		sctl:      sctl,
		mon:       qmpPool,
		tasks:     NewTaskPool(qmpPool, rpcClient),
	}

	rpcSrv := rpc.NewServer()
	rpcSrv.RegisterCodec(jsonrpc.NewCodec(), "application/json")
	rpcSrv.RegisterService(&rpch, "RPC")
	rpcSrv.RegisterValidateRequestFunc(rpch.requestPreHandler)
	rpcSrv.RegisterBeforeFunc(writeLog)
	rpcSrv.RegisterAfterFunc(writeLog)

	r := mux.NewRouter()
	r.Handle("/rpc/v1", rpcSrv)

	// Run servers
	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return rpcserver.ServeUnixSocket(ctx, r, "@/run/kvmrund.sock")
	})
	group.Go(func() error {
		return rpcserver.ServeTls(ctx, r, appConf.Server.BindAddrs, appConf.Common.ServerCrt, appConf.Common.ServerKey)
	})

	return group.Wait()
}

func monitorReConnect(qmpPool *QMPPool, sctl *systemd.Manager) (int, error) {
	var count int

	units, err := sctl.GetAllUnits()
	if err != nil {
		return 0, err
	}

	for _, unit := range units {
		if unit.ActiveState == "active" && unit.SubState == "running" {
			if _, err := qmpPool.NewMonitor(unit.VMName); err == nil {
				count++
			} else {
				log.Errorf("Unable to connect to %s: %s", unit.VMName, err)
			}
		}
	}

	return count, nil
}

func writeLog(i *rpc.RequestInfo) {
	fields := log.Fields{
		"proto":       i.Request.Proto,
		"remote-addr": i.Request.RemoteAddr,
		"request-id":  i.Request.Header.Get("Request-Id"),
	}

	if i.StatusCode == 0 {
		log.WithFields(fields).Info("RPC request: ", i.Method)
	} else {
		fields["code"] = strconv.Itoa(i.StatusCode)
		if i.Error == nil {
			log.WithFields(fields).Info("RPC request: completed")
		} else {
			fields["error"] = i.Error.Error()
			log.WithFields(fields).Error("RPC request: failed")
		}
	}
}
