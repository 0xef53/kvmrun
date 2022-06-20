package system

import (
	"context"
	"net"
	"strings"
	"time"

	pb "github.com/0xef53/kvmrun/api/services/system/v1"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/services"

	qmp "github.com/0xef53/go-qmp/v2"
)

type QemuInstanceShutdownTask struct {
	*task.GenericTask
	*services.ServiceServer

	req *pb.StopQemuInstanceRequest
}

func NewQemuInstanceShutdownTask(req *pb.StopQemuInstanceRequest, ss *services.ServiceServer) *QemuInstanceShutdownTask {
	return &QemuInstanceShutdownTask{
		GenericTask:   new(task.GenericTask),
		ServiceServer: ss,
		req:           req,
	}
}

func (t *QemuInstanceShutdownTask) GetNS() string { return "qemu-shutdown" }

func (t *QemuInstanceShutdownTask) GetKey() string { return t.req.Name + "/system::" }

func (t *QemuInstanceShutdownTask) Main() error {
	t.cancelAllActiveTasks()

	return t.stop()
}

func (t *QemuInstanceShutdownTask) cancelAllActiveTasks() {
	keys := []string{
		"qemu-reqistration:" + t.req.Name + "/system::",
	}

	for _, key := range t.Tasks.List() {
		ff := strings.SplitN(key, ":", 3)
		if len(ff) == 3 && ff[1] == t.req.Name {
			keys = append(keys, key)
		}
	}

	if len(keys) > 0 {
		t.Logger.Info("Forced cancellation all background tasks")
		for _, key := range keys {
			t.Tasks.Cancel(key)
		}
	}
}

func (t *QemuInstanceShutdownTask) stop() error {
	t.Logger.Info("Forced resuming the emulation")

	if err := t.Mon.Run(t.req.Name, qmp.Command{"cont", nil}, nil); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(t.req.GracefulTimeout)*time.Second)
	defer cancel()

	okStopped := make(chan struct{})

	go func() {
		defer close(okStopped)

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		// Repeat SYSTEM_POWERDOWN every second just to be sure that the guest will receive it
		for {
			if err := t.Mon.Run(t.req.Name, qmp.Command{"system_powerdown", nil}, nil); err != nil {
				// In this case we suppose that the socket is closed,
				// i.e. virtual machine is not running. That is what we need
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	select {
	case <-ctx.Done():
		t.Logger.Warn("Timed out: sending quit signal")
		if err := t.Mon.Run(t.req.Name, qmp.Command{"quit", nil}, nil); err != nil {
			if _, ok := err.(*net.OpError); !ok {
				return err
			}
		}
	case <-okStopped:
		t.Logger.Info("Has been terminated")
	}

	return nil
}
