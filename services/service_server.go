package services

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/internal/appconf"
	"github.com/0xef53/kvmrun/internal/monitor"
	"github.com/0xef53/kvmrun/internal/ps"
	"github.com/0xef53/kvmrun/internal/randstring"
	"github.com/0xef53/kvmrun/internal/systemd"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/kvmrun"

	qmp "github.com/0xef53/go-qmp/v2"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	log "github.com/sirupsen/logrus"
	grpc_metadata "google.golang.org/grpc/metadata"
)

type ServiceServer struct {
	AppConf   *appconf.KvmrunConfig
	SystemCtl *systemd.Manager
	Mon       *monitor.Pool
	Tasks     *task.Pool
}

func (s *ServiceServer) GetMachine(vmname string) (*kvmrun.Machine, error) {
	if len(vmname) == 0 {
		return nil, fmt.Errorf("machine name is not defined")
	}

	get := func(vmname string, mon *qmp.Monitor) (*kvmrun.Machine, error) {
		vm, err := kvmrun.GetMachine(vmname, mon)

		switch err.(type) {
		case nil:
		case *net.OpError:
			// If a QEMU process was terminated bypassing kvmrun
			// (for example: SIGTERM / SIGKILL / SIGINT) we will get
			// an error of type *net.OpError with code == syscall.ECONNRESET.
			// As a workaround for this case - just repeat the request with mon = nil
			return kvmrun.GetMachine(vmname, nil)
		default:
			return nil, err
		}

		return vm, nil
	}

	mon, _ := s.Mon.Get(vmname)

	vm, err := get(vmname, mon)
	if err != nil {
		return nil, err
	}

	return vm, nil
}

func (s *ServiceServer) GetMachineStatus(vm *kvmrun.Machine) (kvmrun.InstanceState, error) {
	var vmi kvmrun.Instance

	if vm.R != nil {
		vmi = vm.R
	} else {
		vmi = vm.C
	}

	// Trying to get the special status of instance.
	// Exiting on success.
	if st, err := vmi.Status(); err == nil {
		switch st {
		case kvmrun.StatePaused:
			return st, nil
		case kvmrun.StateIncoming, kvmrun.StateMigrating, kvmrun.StateMigrated:
			return st, nil
		}
	} else {
		return 0, err
	}

	unit, err := s.SystemCtl.GetUnit(s.MachineToUnit(vm.Name))
	if err != nil {
		return kvmrun.StateNoState, fmt.Errorf("systemd dbus request failed: %s", err)
	}

	switch unit.ActiveState {
	case "active":
		switch unit.SubState {
		case "running":
			return kvmrun.StateRunning, nil
		}
	case "inactive":
		return kvmrun.StateInactive, nil
	case "activating":
		return kvmrun.StateStarting, nil
	case "deactivating":
		return kvmrun.StateShutdown, nil
	case "failed":
		return kvmrun.StateCrashed, nil
	}

	return kvmrun.StateNoState, nil
}

func (s *ServiceServer) GetMachineNames() ([]string, error) {
	files, err := os.ReadDir(kvmrun.CONFDIR)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(files))

	for _, f := range files {
		conffile := filepath.Join(kvmrun.CONFDIR, f.Name(), "config")
		if _, err := os.Stat(conffile); err == nil {
			names = append(names, f.Name())
		}
	}

	return names, nil
}

func (s *ServiceServer) GetMachineLifeTime(vm *kvmrun.Machine) (time.Duration, error) {
	if vm.R == nil {
		return 0, nil
	}

	t, err := ps.GetLifeTime(vm.R.Pid())
	if err != nil {
		return 0, err
	}

	return t, nil
}

func (s *ServiceServer) MachineDownFile(vmname string) string {
	return filepath.Join(filepath.Join(kvmrun.CONFDIR, vmname, "down"))
}

func (s *ServiceServer) newContext(ctx context.Context) context.Context {
	var tag string

	if t := grpc_ctxtags.Extract(ctx).Values()["request-tag"]; t != nil {
		tag = t.(string)
	} else if md, ok := grpc_metadata.FromOutgoingContext(ctx); ok {
		if v, ok := md["request-tag"]; ok {
			tag = v[0]
		}
	} else {
		tag = randstring.RandString(8)
	}

	return context.WithValue(context.Background(), "tag", tag)
}

var longRunningTasks = map[string]error{
	"machine-migration": fmt.Errorf("machine is locked because the migration process is currently in progress"),
	"machine-incoming":  fmt.Errorf("machine is locked because the incoming-migration process is currently in progress"),
	"backup":            fmt.Errorf("resource is locked because the backup process is currently in progress"),
}

func (s *ServiceServer) startTask(fn func() (string, error)) (string, error) {
	var key string
	var err error

	for attempt := 0; attempt < 5; attempt++ {
		key, err = fn()

		if _err, ok := err.(*task.TaskAlreadyRunningError); ok {
			// Immediately stop if one of the long tasks is found
			if descErr, found := longRunningTasks[_err.Namespace]; found {
				err = descErr
				break
			}
		} else {
			break
		}

		time.Sleep(time.Second)
	}

	return key, err
}

func (s *ServiceServer) StartTask(ctx context.Context, t task.Task, resp interface{}) (string, error) {
	return s.startTask(func() (string, error) {
		return s.Tasks.StartTask(s.newContext(ctx), t, resp)
	})
}

func (s *ServiceServer) RunFuncTask(ctx context.Context, v string, fn func(*log.Entry) error) error {
	var key string

	if strings.Contains(v, ":") {
		key = v
	} else {
		key = v + ":function:"
	}

	_, err := s.startTask(func() (string, error) {
		return s.Tasks.RunFunc(ctx, key, fn)
	})

	return err
}

func (s *ServiceServer) MachineToUnit(vmname string) string {
	return "kvmrun@" + vmname + ".service"
}

func (s *ServiceServer) ProxyToUnit(vmname, diskname string) string {
	return "kvmrun-proxy@" + vmname + "@" + diskname + ".service"
}

func (s *ServiceServer) ActivateDiskBackendProxy(vmname, diskname string) error {
	unitname := s.ProxyToUnit(vmname, diskname)

	// Enable, start and test
	if err := s.SystemCtl.Enable(unitname); err != nil {
		return err
	}

	return s.SystemCtl.StartAndTest(unitname, 5*time.Second, nil)
}

func (s *ServiceServer) DeactivateDiskBackendProxy(vmname, diskname string) error {
	unitname := s.ProxyToUnit(vmname, diskname)

	if err := s.SystemCtl.StopAndWait(unitname, 5*time.Second, nil); err != nil {
		s.SystemCtl.KillBySIGKILL(unitname)
	}

	return s.SystemCtl.Disable(unitname)
}
