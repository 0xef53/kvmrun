package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/0xef53/kvmrun/pkg/kcfg"
	"github.com/0xef53/kvmrun/pkg/kvmrun"
	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
	rpcserver "github.com/0xef53/kvmrun/pkg/rpc/server"
	"github.com/0xef53/kvmrun/pkg/runsv"

	"github.com/gorilla/mux"
	rpc "github.com/gorilla/rpc/v2"
	jsonrpc "github.com/gorilla/rpc/v2/json2"
)

var (
	KConf *kcfg.KvmrunConfig

	QPool *QMPPool
	IPool *VMInitPool
	MPool *MigrationPool
	TPool *DiskJobPool

	RPCClient *rpcclient.TlsClient

	DebugWriter io.Writer = ioutil.Discard
)

func init() {
	runsv.REPOSITORY = kvmrun.VMCONFDIR
}

func main() {
	confFile := "/etc/kvmrun/kvmrun.ini"

	flag.StringVar(&confFile, "config", confFile, "path to the config `file`")
	flag.Parse()

	if _, ok := os.LookupEnv("DEBUG"); ok {
		DebugWriter = os.Stdout
	}

	if c, err := kcfg.NewConfig(confFile); err == nil {
		KConf = c
	} else {
		log.Fatalln(err)
	}

	QPool = NewQMPPool(kvmrun.QMPMONDIR)
	IPool = NewVMInitPool()
	MPool = NewMigrationPool()
	TPool = NewDiskJobPool()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		log.Println(<-sigc)
		cancel() // this prevents any new tasks
		// TODO: take background tasks into account (wg?)
	}()

	// Secure client
	if c, err := rpcclient.NewTlsClient("/rpc/v1", KConf.Common.ClientCrt, KConf.Common.ClientKey); err == nil {
		RPCClient = c
	} else {
		log.Fatalln(err)
	}

	// Server
	rpcSrv := rpc.NewServer()
	rpcSrv.RegisterCodec(jsonrpc.NewCodec(), "application/json")
	rpcSrv.RegisterService(&RPC{}, "")
	rpcSrv.RegisterValidateRequestFunc(requestPreHandler)

	r := mux.NewRouter()
	r.Handle("/rpc/v1", rpcSrv)
	r.Use(logging)

	bindAddrs := make([]string, 0, len(KConf.Server.BindAddrs))

	for _, addr := range KConf.Server.BindAddrs {
		if addr.To4() != nil {
			bindAddrs = append(bindAddrs, addr.String()+":9393")
		} else {
			bindAddrs = append(bindAddrs, "["+addr.String()+"]:9393")
		}
	}

	// Try to re-create QMP pool for running virt.machines
	if n, err := monitorReConnect(); err == nil {
		log.Printf("Found %d running instances\n", n)
	} else {
		log.Fatalln(err)
	}

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return rpcserver.ServeUnixSocket(ctx, r, "@/run/kvmrund.sock")
	})
	group.Go(func() error {
		return rpcserver.ServeTls(ctx, r, bindAddrs, KConf.Common.ServerCrt, KConf.Common.ServerKey)
	})

	if err := group.Wait(); err != nil {
		log.Fatal(err)
	}
}

func monitorReConnect() (int, error) {
	var count int

	vms, err := getVMNames()
	if err != nil {
		return 0, err
	}

	for _, vmname := range vms {
		st, err := runsv.GetState(vmname)
		if err != nil {
			return 0, err
		}
		if st == "run" {
			if _, err := QPool.NewMonitor(vmname); err == nil {
				count++
			} else {
				log.Printf("Unable to connect to %s: %s\n", vmname, err)
			}
		}
	}

	return count, nil
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf(
			"proto=%s method=%s remote-addr=%s endpoint=%s command=%s\n",
			r.Proto,
			r.Method,
			r.RemoteAddr,
			r.URL,
			r.Header.Get("Method-Name"),
		)

		next.ServeHTTP(w, r)
	})
}

type RPC struct{}

// ReleaseResources cleans all resources allocated for the virtual machine.
// All background tasks will be gracefully interrupted.
// This function should be called before the QEMU process begins to stop.
func (x *RPC) ReleaseResources(r *http.Request, args *rpccommon.VMNameRequest, resp *struct{}) error {
	IPool.Release(args.Name)
	MPool.Release(args.Name)
	QPool.CloseMonitor(args.Name)
	return nil
}

type InterruptedError struct {
	Err error
}

func (e *InterruptedError) Error() string {
	if e.Err != nil {
		return "Interrupted error: " + e.Err.Error()
	}
	return "Process was interrupted"
}

func NewInterruptedError(format string, a ...interface{}) error {
	return &InterruptedError{fmt.Errorf(format, a...)}
}

func IsInterruptedError(err error) bool {
	if _, ok := err.(*InterruptedError); ok {
		return true
	}
	return false
}
