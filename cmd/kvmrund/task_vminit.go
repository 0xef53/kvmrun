package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/pkg/ps"
	qt "github.com/0xef53/kvmrun/pkg/qemu/types"

	qmp "github.com/0xef53/go-qmp/v2"
	"golang.org/x/sync/errgroup"
)

type VMInitTaskOpts struct {
	VMName    string
	Pid       int
	MemActual uint64
}

type VMInitTask struct {
	task
	opts *VMInitTaskOpts
}

func (t *VMInitTask) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel != nil {
		return fmt.Errorf("already running")
	}

	t.released = make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	// Main  process
	go func() {
		var err error

		t.logger.Info("Starting")

		defer func() {
			t.mu.Lock()
			t.cancel = nil
			t.err = err
			close(t.released)
			t.mu.Unlock()
		}()

		switch err = t.initialize(ctx); {
		case err == nil:
			t.completed = true
			t.logger.Info("Successfully completed")
		case IsTaskInterruptedError(err):
			t.logger.Warn("Interrupted by the CANCEL command")
		default:
			t.logger.Errorf("Fatal error: %s", err)
		}
	}()

	return nil
}

func (t *VMInitTask) initialize(ctx context.Context) error {
	t.logger.Debug("initialize(): main process started")

	t.logger.Debug("initialize(): wait for qemu-system process")
	if err := t.waitForQemuSystem(); err != nil {
		return err
	}

	group1, ctx1 := errgroup.WithContext(ctx)

	if t.opts.MemActual > 0 {
		group1.Go(func() error { return t.initBalloon(ctx1) })
	}

	if err := group1.Wait(); err != nil {
		return err
	}

	t.logger.Debug("initialize(): completed")

	return nil
}

func (t *VMInitTask) initBalloon(ctx context.Context) error {
	t.logger.Debug("initBalloon(): starting ...")

	ErrNotSuccessfully := errors.New("attempt failed")

	set := func() error {
		balloonInfo := struct {
			Actual uint64 `json:"actual"`
		}{}

		if err := t.mon.Run(t.opts.VMName, qmp.Command{"balloon", &qt.Uint64Value{t.opts.MemActual}}, nil); err != nil {
			return err
		}

		if err := t.mon.Run(t.opts.VMName, qmp.Command{"query-balloon", nil}, &balloonInfo); err != nil {
			return err
		}

		if t.opts.MemActual != balloonInfo.Actual {
			return ErrNotSuccessfully
		}

		return nil
	}

	timeout := 3

	for i := 0; i < 180; i++ {
		select {
		case <-ctx.Done():
			return &TaskInterruptedError{}
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
func (t *VMInitTask) waitForQemuSystem() error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		}
		pline, err := ps.GetCmdline(t.opts.Pid)
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
	if _, err := t.mon.NewMonitor(t.opts.VMName); err != nil {
		return fmt.Errorf("QEMU init process failed: %s", err)
	}

	return nil
}
