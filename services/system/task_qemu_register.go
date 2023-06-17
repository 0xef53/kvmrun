package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/internal/helpers"
	"github.com/0xef53/kvmrun/internal/ps"
	qemu_types "github.com/0xef53/kvmrun/internal/qemu/types"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/services"

	pb "github.com/0xef53/kvmrun/api/services/system/v1"

	qmp "github.com/0xef53/go-qmp/v2"
	log "github.com/sirupsen/logrus"
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

	group.Go(func() error { return t.initNetworkSecondStage(ctx) })

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
	t.Logger.Info("[memory] Start the memory balloon configuring")

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
		t.Logger.Infof("[memory] Actual memory size is set to %d MB", t.req.MemActual>>20)

	} else {
		t.Logger.Errorf("[memory] Failed to set actual memory size via virtio_balloon: %s", err)
	}

	return nil
}

func (t *QemuInstanceRegistrationTask) initNetworkSecondStage(ctx context.Context) error {
	t.Logger.Info("[network] Start the second stage network configuring")

	ts := time.Now()

	var st qemu_types.StatusInfo

	if err := t.Mon.Run(t.req.Name, qmp.Command{"query-status", nil}, &st); err != nil {
		return err
	}

	if !st.Running {
		t.Logger.Infof("[network] Wait for machine emulation to be running. Current status is %s", st.Status)

		if _, err := t.Mon.WaitMachineResumeStateEvent(t.req.Name, ctx, uint64(ts.Unix())); err != nil {
			return err
		}
	}

	cfg := struct {
		Ifaces []kvmrun.NetIface `json:"network"`
	}{}

	if b, err := ioutil.ReadFile(filepath.Join(kvmrun.CONFDIR, t.req.Name, "config")); err == nil {
		if err := json.Unmarshal(b, &cfg); err != nil {
			return err
		}
	} else {
		if os.IsNotExist(err) {
			return &kvmrun.NotFoundError{t.req.Name}
		} else {
			return err
		}
	}

	isSupported := func(binary string) bool {
		if err := exec.Command(binary, "--test-second-stage-feature").Run(); err != nil {
			return false
		}
		return true
	}

	for _, netif := range cfg.Ifaces {
		if len(netif.Ifup) == 0 {
			continue
		}

		t.Logger.Infof("[network] Start IFUP-script for interface %s", netif.Ifname)

		if _, err := helpers.ResolveExecutable(netif.Ifup); err == nil {
			if isSupported(netif.Ifup) {
				ifupCmd := exec.Command(netif.Ifup, "--second-stage", netif.Ifname)

				ifupCmd.Dir = filepath.Join(kvmrun.CONFDIR, t.req.Name)

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
