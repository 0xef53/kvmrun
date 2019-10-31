package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	qmp "github.com/0xef53/go-qmp"

	"github.com/0xef53/kvmrun/pkg/ps"
	qt "github.com/0xef53/kvmrun/pkg/qemu/types"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (x *RPC) InitQemuInstance(r *http.Request, args *rpccommon.QemuInitRequest, resp *struct{}) error {
	ii, err := IPool.Get(args.Name)
	if err != nil {
		return err
	}

	opts := VMInitOpts{
		Pid:       args.Pid,
		MemActual: args.MemActual,
	}

	return ii.Start(&opts)
}

//
// POOL
//

type VMInitPool struct {
	mu    sync.Mutex
	table map[string]*VMInit
}

func NewVMInitPool() *VMInitPool {
	p := VMInitPool{}
	p.table = make(map[string]*VMInit)
	return &p
}

func (p *VMInitPool) Get(vmname string) (*VMInit, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if entry, found := p.table[vmname]; found {
		return entry, nil
	}

	entry := &VMInit{vmname: vmname}

	p.table[vmname] = entry

	return entry, nil
}

func (p *VMInitPool) Exists(vmname string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, found := p.table[vmname]; found {
		return true
	}

	return false
}

func (p *VMInitPool) release(vmname string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if entry, found := p.table[vmname]; found {
		// Cancel the existing process
		entry.Cancel()
		// ... and wait for it to be done
		<-entry.released

		delete(p.table, vmname)
	}
}

func (p *VMInitPool) Release(vmname string) {
	p.release(vmname)
}

//
// OPTS
//

type VMInitOpts struct {
	Pid       int
	MemActual uint64
}

//
// MAIN
//

type VMInit struct {
	mu sync.Mutex

	vmname string
	opts   *VMInitOpts

	completed bool
	cancel    context.CancelFunc
	released  chan struct{}

	err error
}

func (ii *VMInit) printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf("[vm_init:"+ii.vmname+"] "+format+"\n", a...)
}

func (ii *VMInit) debugf(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(DebugWriter, "[vm_init:"+ii.vmname+"] DEBUG: "+format+"\n", a...)
}

// Err returns last migration error
func (ii *VMInit) Err() error {
	ii.mu.Lock()
	defer ii.mu.Unlock()

	return ii.err
}

func (ii *VMInit) Cancel() error {
	ii.mu.Lock()
	defer ii.mu.Unlock()

	if ii.cancel == nil {
		return fmt.Errorf("init process is not running")
	}

	ii.cancel()

	return nil
}

func (ii *VMInit) Start(opts *VMInitOpts) error {
	ii.mu.Lock()
	defer ii.mu.Unlock()

	if ii.cancel != nil {
		return fmt.Errorf("init process is already running")
	}

	ii.opts = opts

	ii.released = make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	ii.cancel = cancel

	// Main  process
	go func() {
		var err error

		ii.printf("Starting")

		defer func() {
			ii.mu.Lock()
			ii.cancel = nil
			ii.err = err
			close(ii.released)
			ii.mu.Unlock()
		}()

		switch err = ii.initialize(ctx); {
		case err == nil:
			ii.completed = true
			ii.printf("Successfully completed")
		case IsInterruptedError(err):
			ii.printf("Interrupted by the CANCEL command")
		default:
			ii.printf("Fatal error: %s", err)
		}
	}()

	return nil
}

func (ii *VMInit) initialize(ctx context.Context) error {
	ii.debugf("initialize(): main process started")

	ii.debugf("initialize(): wait for qemu-system process")
	if err := ii.waitForQemuSystem(); err != nil {
		return err
	}

	group1, ctx1 := errgroup.WithContext(ctx)

	if ii.opts.MemActual > 0 {
		group1.Go(func() error { return ii.initBalloon(ctx1) })
	}

	if err := group1.Wait(); err != nil {
		return err
	}

	ii.debugf("initialize(): completed")

	return nil
}

func (ii *VMInit) initBalloon(ctx context.Context) error {
	ii.debugf("initBalloon(): starting ...")

	ErrNotSuccessfully := errors.New("attempt failed")

	set := func() error {
		balloonInfo := struct {
			Actual uint64 `json:"actual"`
		}{}

		if err := QPool.Run(ii.vmname, qmp.Command{"balloon", &qt.Uint64Value{ii.opts.MemActual}}, nil); err != nil {
			return err
		}

		if err := QPool.Run(ii.vmname, qmp.Command{"query-balloon", nil}, &balloonInfo); err != nil {
			return err
		}

		if ii.opts.MemActual != balloonInfo.Actual {
			return ErrNotSuccessfully
		}

		return nil
	}

	timeout := 3

	for i := 0; i < 180; i++ {
		select {
		case <-ctx.Done():
			return &InterruptedError{}
		default:
			time.Sleep(time.Second * time.Duration(timeout))
		}

		if i == 60 {
			timeout = 10
		}

		switch err := set(); {
		case err == nil:
			return nil
		case err == ErrNotSuccessfully:
			// Retrying
			continue
		case qmp.IsSocketNotAvailable(err):
			// Retrying
			continue
		default:
			return err
		}
	}

	return fmt.Errorf("failed to set actual memory size via virtio_balloon")
}

// waitForQemuSystem waits until the "launcher" process turns
// into the "qemu-system" process.
func (ii *VMInit) waitForQemuSystem() error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		}
		pline, err := ps.GetCmdline(ii.opts.Pid)
		switch {
		case err == nil:
		case os.IsNotExist(err):
			// ID has been changed.
			// This means that "launcher" or "qemu-system" process failed.
			return fmt.Errorf("QEMU init process failed: unable to start")
		default:
			return err
		}
		if len(pline) != 0 && strings.HasPrefix(filepath.Base(pline[0]), "qemu-system") {
			break
		}
	}
	if _, err := QPool.NewMonitor(ii.vmname); err != nil {
		return fmt.Errorf("QEMU init process failed: %s", err)
	}

	return nil
}
