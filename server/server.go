package server

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/0xef53/kvmrun/internal/appconf"
	"github.com/0xef53/kvmrun/internal/monitor"
	"github.com/0xef53/kvmrun/internal/systemd"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/internal/task/classifiers"
	"github.com/0xef53/kvmrun/kvmrun"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type Server struct {
	SessionID string

	AppConf   *appconf.Config
	SystemCtl *systemd.Manager
	Mon       *monitor.Pool
	Tasks     *task.Pool
}

func NewServer(_ context.Context, appConf *appconf.Config) (*Server, error) {
	if appConf == nil {
		return nil, fmt.Errorf("empty application config")
	}

	if appConf.TLSConfig == nil || appConf.Server.TLSConfig == nil {
		return nil, fmt.Errorf("both server and client TLS certificates must exist")
	}

	if _, err := os.Stat(appConf.Kvmrun.QemuRootDir); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("QEMU root directory does not exist: %s", appConf.Kvmrun.QemuRootDir)
		}
		return nil, fmt.Errorf("failed to check QEMU root directory: %w", err)
	}

	if appConf.Kvmrun.QemuRootDir != kvmrun.DEFAULT_QEMU_ROOTDIR {
		log.Infof("QEMU root directory: %s", appConf.Kvmrun.QemuRootDir)
	}

	srv := Server{
		SessionID: uuid.New().String(),
		AppConf:   appConf,
		Mon:       monitor.NewPool(kvmrun.QMPMONDIR),
		Tasks:     task.NewPool(),
	}

	// Systemd connection
	if ctl, err := systemd.NewManager(); err == nil {
		srv.SystemCtl = ctl
	} else {
		return nil, err
	}

	// Pool of background tasks
	if srv.Tasks != nil {
		uniqueLabelCls := classifiers.NewUniqueLabelClassifier()

		if _, err := srv.Tasks.RegisterClassifier(uniqueLabelCls, "unique-labels"); err != nil {
			return nil, err
		}

		groupLabelCls := classifiers.NewGroupLabelClassifier()

		if _, err := srv.Tasks.RegisterClassifier(groupLabelCls, "group-labels"); err != nil {
			return nil, err
		}

		limitedGroupCls := classifiers.NewLimitedGroupClassifier("hostnet", 2, 3600*time.Second)

		if _, err := srv.Tasks.RegisterClassifier(limitedGroupCls, "hostnet-group"); err != nil {
			return nil, err
		}
	}

	// Try to re-create QMP pool for running virt.machines
	if n, err := srv.monitorReConnect(); err == nil {
		if n == 1 {
			log.Infof("Found %d running instance", n)
		} else {
			log.Infof("Found %d running instances", n)
		}
	} else {
		return nil, err
	}

	return &srv, nil
}

func (s *Server) monitorReConnect() (int, error) {
	unit2vm := func(unitname string) string {
		return strings.TrimSuffix(strings.TrimPrefix(unitname, "kvmrun@"), ".service")
	}

	var count int

	units, err := s.SystemCtl.GetAllUnits("kvmrun@*.service")
	if err != nil {
		return 0, err
	}

	for _, unit := range units {
		if unit.ActiveState == "active" && unit.SubState == "running" {
			if _, err := s.Mon.NewMonitor(unit2vm(unit.Name)); err == nil {
				count++
			} else {
				log.Errorf("Unable to connect to %s: %s", unit2vm(unit.Name), err)
			}
		}
	}

	return count, nil
}

func (s *Server) GetAppConfig(_ context.Context) *appconf.Config {
	return &appconf.Config{
		Kvmrun: appconf.KvmrunConfig{
			QemuRootDir: s.AppConf.Kvmrun.QemuRootDir,
			CertDir:     s.AppConf.Kvmrun.CertDir,
		},
	}
}

func (s *Server) GracefulShutdown(_ context.Context) error {
	log.Info("A graceful shutdown requested. SIGTEM will be sent to the kvmrund process")

	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	time.Sleep(3 * time.Second)

	return nil
}
