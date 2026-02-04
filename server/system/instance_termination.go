package system

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/0xef53/kvmrun/server"

	qmp "github.com/0xef53/go-qmp/v2"
	"github.com/0xef53/go-task"
)

type InstanceTerminationOptions struct {
	GracefulTimeout time.Duration `json:"graceful_timeout"`
}

func (o *InstanceTerminationOptions) Validate(strict bool) error {
	if strict {
		if o.GracefulTimeout < 5*time.Second {
			return fmt.Errorf("graceful shutdown timeout cannot be less than 5 seconds")
		}
	}

	return nil
}

func (s *Server) StartInstanceTermination(ctx context.Context, vmname string, opts *InstanceTerminationOptions) (string, error) {
	if opts == nil {
		return "", fmt.Errorf("empty instance termination opts")
	}

	t := NewInstanceTerminationTask(vmname, opts)

	t.Server = s

	tid, err := s.TaskStart(ctx, t, nil, server.WithUniqueLabel(vmname+"/shutdown"))
	if err != nil {
		return "", fmt.Errorf("cannot start instance termination: %w", err)
	}

	return tid, nil
}

type InstanceTerminationTask struct {
	*task.GenericTask
	*Server

	targets map[string]task.OperationMode

	// Arguments
	vmname string
	opts   *InstanceTerminationOptions
}

func NewInstanceTerminationTask(vmname string, opts *InstanceTerminationOptions) *InstanceTerminationTask {
	return &InstanceTerminationTask{
		GenericTask: new(task.GenericTask),
		targets:     server.BlockAnyOperations(vmname + "/shutdown"),
		vmname:      vmname,
		opts:        opts,
	}
}

func (t *InstanceTerminationTask) Targets() map[string]task.OperationMode { return t.targets }

func (t *InstanceTerminationTask) BeforeStart(_ interface{}) (err error) {
	if t.opts == nil {
		return fmt.Errorf("empty instance termination opts")
	} else {
		if err := t.opts.Validate(true); err != nil {
			return err
		}
	}

	return nil
}

func (t *InstanceTerminationTask) Main() error {
	t.Logger.Info("Forced cancellation all background tasks")

	// Using group label classifier with label == vmname
	t.TaskCancel(t.vmname)

	return t.terminate()
}

func (t *InstanceTerminationTask) terminate() error {
	if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "cont", Arguments: nil}, nil); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.opts.GracefulTimeout)
	defer cancel()

	okStopped := make(chan struct{})

	go func() {
		defer close(okStopped)

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		// Repeat SYSTEM_POWERDOWN every second just to be sure that the guest will receive it
		for {
			if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "system_powerdown", Arguments: nil}, nil); err != nil {
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

		if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "quit", Arguments: nil}, nil); err != nil {
			if _, ok := err.(*net.OpError); !ok {
				return err
			}
		}
	case <-okStopped:
		t.Logger.Info("Has been terminated")
	}

	return nil
}
