package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	qemu_types "github.com/0xef53/kvmrun/internal/qemu/types"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/internal/utils"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	qmp "github.com/0xef53/go-qmp/v2"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type InstanceRegistrationOptions struct {
	MemActual uint32 `json:"mem_actual"`
	PID       uint32 `json:"pid"`
}

func (o *InstanceRegistrationOptions) Validate(_ bool) error {
	if o.MemActual == 0 {
		return fmt.Errorf("invalid actual memory value: must be greater than 0")
	}

	if o.PID == 0 {
		return fmt.Errorf("invalid PID value: must be greater than 0")
	}

	return nil
}

func (s *Server) StartInstanceRegistration(ctx context.Context, vmname string, opts *InstanceRegistrationOptions) (string, error) {
	if opts == nil {
		return "", fmt.Errorf("empty instance registration opts")
	}

	t := NewInstanceRegistrationTask(vmname, opts)

	t.Server = s

	taskOpts := []task.TaskOption{
		server.WithUniqueLabel(vmname + "/registration"),
		server.WithGroupLabel(vmname),
	}

	tid, err := s.TaskStart(ctx, t, nil, taskOpts...)
	if err != nil {
		return "", fmt.Errorf("cannot start instance registration: %w", err)
	}

	return tid, nil
}

type InstanceRegistrationTask struct {
	*task.GenericTask
	*Server

	targets map[string]task.OperationMode

	// Arguments
	vmname string
	opts   *InstanceRegistrationOptions
}

func NewInstanceRegistrationTask(vmname string, opts *InstanceRegistrationOptions) *InstanceRegistrationTask {
	return &InstanceRegistrationTask{
		GenericTask: new(task.GenericTask),
		targets:     server.BlockAnyOperations(vmname + "/registration"),
		vmname:      vmname,
		opts:        opts,
	}
}

func (t *InstanceRegistrationTask) Targets() map[string]task.OperationMode { return t.targets }

func (t *InstanceRegistrationTask) BeforeStart(_ interface{}) (err error) {
	if t.opts == nil {
		return fmt.Errorf("empty instance registration opts")
	} else {
		if err := t.opts.Validate(true); err != nil {
			return err
		}
	}

	return nil
}

func (t *InstanceRegistrationTask) Main() error {
	if err := t.waitForQemuSystem(); err != nil {
		return fmt.Errorf("qemu-system process start failed: %w", err)
	}

	if _, err := t.Mon.NewMonitor(t.vmname); err != nil {
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

			if pline, err := utils.GetCmdline(int(t.opts.PID)); err == nil && len(pline) > 0 {
				if strings.HasPrefix(filepath.Base(pline[0]), "qemu-system") {
					continue
				}
			}

			break
		}

		t.Logger.Error("QEMU process exited before the task was completed. Canceling the task")

		t.Cancel()
	}()

	if t.opts.MemActual > 0 {
		group.Go(func() error { return t.initBalloon(ctx) })
	}

	group.Go(func() error { return t.initNetworkSecondStage(ctx) })

	return group.Wait()
}

// waitForQemuSystem waits until the "launcher" process turns
// into the "qemu-system" process.
func (t *InstanceRegistrationTask) waitForQemuSystem() error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C

		pline, err := utils.GetCmdline(int(t.opts.PID))
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

func (t *InstanceRegistrationTask) initBalloon(ctx context.Context) error {
	t.Logger.Info("[memory] Start the memory balloon configuring")

	set := func() (bool, error) {
		req := qemu_types.Uint64Value{
			Value: uint64(t.opts.MemActual),
		}

		if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "balloon", Arguments: &req}, nil); err != nil {
			return false, err
		}

		balloonInfo := struct {
			Actual int64 `json:"actual"`
		}{}

		if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "query-balloon", Arguments: nil}, &balloonInfo); err != nil {
			return false, err
		}

		if t.opts.MemActual != uint32(balloonInfo.Actual) {
			return false, nil
		}

		return true, nil
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

			ok, err := set()

			if (err == nil && !ok) || qmp.IsSocketNotAvailable(err) {
				continue
			}

			// may be nil
			return err
		}

		return fmt.Errorf("backend is not responding")
	}()

	if err == nil {
		t.Logger.Infof("[memory] Actual memory size is set to %d MB", t.opts.MemActual>>20)

	} else {
		t.Logger.Errorf("[memory] Failed to set actual memory size via virtio_balloon: %s", err)
	}

	return nil
}

func (t *InstanceRegistrationTask) initNetworkSecondStage(ctx context.Context) error {
	t.Logger.Info("[network] Start the second stage network configuring")

	ts := time.Now()

	var st qemu_types.StatusInfo

	if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "query-status", Arguments: nil}, &st); err != nil {
		return err
	}

	if !st.Running {
		t.Logger.Infof("[network] Wait for machine emulation to be running. Current status is %s", st.Status)

		if _, err := t.Mon.WaitMachineResumeStateEvent(t.vmname, ctx, uint64(ts.Unix())); err != nil {
			return err
		}
	}

	vmconf, err := kvmrun.GetInstanceConf(t.vmname)
	if err != nil {
		return err
	}

	isSupported := func(binary string) bool {
		if err := exec.Command(binary, "--test-second-stage-feature").Run(); err != nil {
			return false
		}
		return true
	}

	for _, netif := range vmconf.NetIfaceGetList() {
		if len(netif.Ifup) == 0 {
			continue
		}

		t.Logger.Infof("[network] Start IFUP-script for interface %s", netif.Ifname)

		if _, err := utils.ResolveExecutable(netif.Ifup); err == nil {
			if isSupported(netif.Ifup) {
				ifupCmd := exec.Command(netif.Ifup, "--second-stage", netif.Ifname)

				ifupCmd.Dir = filepath.Join(kvmrun.CONFDIR, t.vmname)

				ifupCmd.Stdout = t.Logger.WriterLevel(log.InfoLevel)
				ifupCmd.Stderr = t.Logger.WriterLevel(log.ErrorLevel)

				if err := ifupCmd.Run(); err != nil {
					t.Logger.Errorf("[network] IFUP-script for interface %s failed: %s", netif.Ifname, err)
				}
			}
		} else {
			t.Logger.Errorf("[network] Unable to run IFUP-script for interface %s: %s", netif.Ifname, err)
		}
	}

	return nil
}
