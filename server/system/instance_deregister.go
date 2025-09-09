package system

import (
	"context"
	"fmt"

	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/server"
)

func (s *Server) StartInstanceDeregistration(ctx context.Context, vmname string) (string, error) {
	t := NewInstanceDeregistrationTask(vmname)

	t.Server = s

	tid, err := s.TaskStart(ctx, t, nil, server.WithUniqueLabel(vmname+"/deregistration"))
	if err != nil {
		return "", fmt.Errorf("cannot start instance deregistration: %w", err)
	}

	return tid, nil
}

type InstanceDeregistrationTask struct {
	*task.GenericTask
	*Server

	targets map[string]task.OperationMode

	// Arguments
	vmname string
}

func NewInstanceDeregistrationTask(vmname string) *InstanceDeregistrationTask {
	return &InstanceDeregistrationTask{
		GenericTask: new(task.GenericTask),
		targets:     server.BlockAnyOperations(vmname + "/deregistration"),
		vmname:      vmname,
	}
}

func (t *InstanceDeregistrationTask) Targets() map[string]task.OperationMode { return t.targets }

func (t *InstanceDeregistrationTask) BeforeStart(_ interface{}) (err error) {
	t.Server.Mon.CloseMonitor(t.vmname)

	return nil
}

func (t *InstanceDeregistrationTask) Main() error {
	// no background actions yet
	return nil
}
