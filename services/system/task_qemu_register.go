package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/internal/ps"
	qemu_types "github.com/0xef53/kvmrun/internal/qemu/types"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/services"

	pb "github.com/0xef53/kvmrun/api/services/system/v1"

	qmp "github.com/0xef53/go-qmp/v2"
	"golang.org/x/sync/errgroup"
)

type QemuInstanceRegistrationTask struct {
	*task.GenericTask
	*services.ServiceServer

	req *pb.RegisterQemuInstanceRequest
}

func NewQemuInstanceRegistrationTask(req *pb.RegisterQemuInstanceRequest, ss *services.ServiceServer) *QemuInstanceRegistrationTask {
	return &QemuInstanceRegistrationTask{
		GenericTask:   new(task.GenericTask),
		ServiceServer: ss,
		req:           req,
	}
}

func (t *QemuInstanceRegistrationTask) GetNS() string { return "qemu-reqistration" }

func (t *QemuInstanceRegistrationTask) GetKey() string { return t.req.Name + "/system::" }

func (t *QemuInstanceRegistrationTask) Main() error {
	t.Logger.Debug("Wait for qemu-system process")

	if err := t.waitForQemuSystem(); err != nil {
		return fmt.Errorf("qemu-system process start failed: %s", err)
	}

	if _, err := t.Mon.NewMonitor(t.req.Name); err != nil {
		return err
	}

	group, ctx := errgroup.WithContext(t.Ctx())

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			pline, err := ps.GetCmdline(int(t.req.PID))

			if err == nil && len(pline) > 0 && strings.HasPrefix(filepath.Base(pline[0]), "qemu-system") {
				continue
			}

			break
		}

		t.Logger.Warn("QEMU process exited before the task was completed. Canceling the task")

		t.Cancel()
	}()

	if t.req.MemActual > 0 {
		group.Go(func() error { return t.initBalloon(ctx) })
	}

	return group.Wait()
}

// waitForQemuSystem waits until the "launcher" process turns
// into the "qemu-system" process.
func (t *QemuInstanceRegistrationTask) waitForQemuSystem() error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		}

		pline, err := ps.GetCmdline(int(t.req.PID))
		switch {
		case err == nil:
		case os.IsNotExist(err):
			// This means that "launcher" or "qemu-system" process failed
			return fmt.Errorf("PID was changed")
		default:
			return err
		}

		if len(pline) != 0 && strings.HasPrefix(filepath.Base(pline[0]), "qemu-system") {
			break
		}
	}

	return nil
}

func (t *QemuInstanceRegistrationTask) initBalloon(ctx context.Context) error {
	t.Logger.Debug("Request the memory balloon driver")

	errTryAgainLater := errors.New("attempt failed: try again later")

	set := func() error {
		balloonInfo := struct {
			Actual int64 `json:"actual"`
		}{}

		if err := t.Mon.Run(t.req.Name, qmp.Command{"balloon", &qemu_types.Uint64Value{uint64(t.req.MemActual)}}, nil); err != nil {
			return err
		}

		if err := t.Mon.Run(t.req.Name, qmp.Command{"query-balloon", nil}, &balloonInfo); err != nil {
			return err
		}

		if t.req.MemActual != balloonInfo.Actual {
			return errTryAgainLater
		}

		return nil
	}

	err := func() error {
		timeout := 3

		for i := 0; i < 180; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				time.Sleep(time.Duration(timeout) * time.Second)
			}

			if i == 60 {
				timeout = 10
			}

			switch err := set(); {
			case err == errTryAgainLater:
				continue
			case qmp.IsSocketNotAvailable(err):
				continue
			default:
				return err
			}
		}

		return nil
	}()

	if err == nil {
		t.Logger.Infof("Actual memory size is set to %d MB", t.req.MemActual>>20)

	} else {
		t.Logger.Errorf("Failed to set actual memory size via virtio_balloon: %s", err)
	}

	return nil
}
