package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/0xef53/kvmrun/internal/appconf"
	"github.com/0xef53/kvmrun/internal/grpcserver"
	"github.com/0xef53/kvmrun/internal/monitor"
	"github.com/0xef53/kvmrun/internal/systemd"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/kvmrun"

	"github.com/0xef53/kvmrun/services"
	_ "github.com/0xef53/kvmrun/services/cloudinit"
	_ "github.com/0xef53/kvmrun/services/hardware"
	_ "github.com/0xef53/kvmrun/services/machines"
	_ "github.com/0xef53/kvmrun/services/network"
	_ "github.com/0xef53/kvmrun/services/system"
	_ "github.com/0xef53/kvmrun/services/tasks"

	cg "github.com/0xef53/go-cgroups"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true,
	})

	app := cli.NewApp()
	app.Usage = "GRPC/REST interface for managing virtual machines"
	app.Action = run
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Usage:   "path to the configuration file",
			EnvVars: []string{"KVMRUND_CONFIG"},
			Value:   "/etc/kvmrun/kvmrun.ini",
		},
		&cli.BoolFlag{
			Name:    "debug",
			Usage:   "print debug information",
			EnvVars: []string{"KVMRUND_DEBUG", "DEBUG"},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalln(err)
	}
}

func run(c *cli.Context) error {
	if c.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	appConf, err := appconf.NewConfig(c.String("config"))
	if err != nil {
		return err
	}

	if appConf.Common.TLSConfig == nil || appConf.Server.TLSConfig == nil {
		return fmt.Errorf("both server and client TLS certificates must exist")
	}

	systemctl, err := systemd.NewManager()
	if err != nil {
		return err
	}

	// Pool of background tasks
	tasks := task.NewPool(3)

	// Pool of all running virt.machines
	mon := monitor.NewPool(kvmrun.QMPMONDIR)

	inner := &services.ServiceServer{
		AppConf:   appConf,
		SystemCtl: systemctl,
		Mon:       mon,
		Tasks:     tasks,
	}

	for _, s := range services.Services() {
		if x, ok := s.(interface{ Init(*services.ServiceServer) }); ok {
			x.Init(inner)
		} else {
			return fmt.Errorf("invalid service interface: %T", s)
		}
	}

	// Main server
	srv := grpcserver.NewServer(&appConf.Server, services.Services())

	// Try to re-create QMP pool for running virt.machines
	if n, err := monitorReConnect(systemctl, mon); err == nil {
		if n == 1 {
			log.Infof("Found %d running instance", n)
		} else {
			log.Infof("Found %d running instances", n)
		}
	} else {
		return err
	}

	// This global cancel context is used by the graceful shutdown function
	cancelCtx, cancel := context.WithCancel(context.Background())

	// Signal handler
	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGTERM, syscall.SIGINT)
		defer signal.Stop(sigc)

		s := <-sigc

		log.WithField("signal", s).Info("Graceful shutdown initiated ...")

		for {
			n := len(tasks.List())

			if n == 0 {
				tasks.WaitAndClosePool()
				cancel()
				break
			}

			log.Warnf("Wait until all tasks finish (currently running: %d). Next attempt in 5 seconds", n)

			time.Sleep(5 * time.Second)
		}
	}()

	// Run unix socket and tcp servers
	return srv.ListenAndServe(cancelCtx)
}

func monitorReConnect(systemctl *systemd.Manager, mon *monitor.Pool) (int, error) {
	unit2vm := func(unitname string) string {
		return strings.TrimSuffix(strings.TrimPrefix(unitname, "kvmrun@"), ".service")
	}

	var count int
	var names []string

	units, err := systemctl.GetAllUnits("kvmrun@*.service")
	if err != nil {
		return 0, err
	}

	for _, unit := range units {
		if unit.ActiveState == "active" && unit.SubState == "running" {
			if _, err := mon.NewMonitor(unit2vm(unit.Name)); err == nil {
				names = append(names, unit2vm(unit.Name))
				count++
			} else {
				log.Errorf("Unable to connect to %s: %s", unit2vm(unit.Name), err)
			}
		}
	}

	if cpuMP, err := cg.GetSubsystemMountpoint("cpu"); err == nil {
		reEnableCPU := func(vmname string) error {
			relpath := filepath.Join(kvmrun.CGROOTPATH, vmname)
			pidfile := filepath.Join(kvmrun.CHROOTDIR, vmname, "pid")

			pid, err := ioutil.ReadFile(pidfile)
			if err != nil {
				return err
			}

			return ioutil.WriteFile(filepath.Join(cpuMP, relpath, "tasks"), pid, 0644)
		}

		for _, vmname := range names {
			if err := reEnableCPU(vmname); err != nil {
				log.Errorf("Unable to re-add '%s' to the CPU control group: %s", vmname, err)
			}
		}
	} else {
		log.Errorf("Unable to initialize 'cpu' controller: %s", err)
	}

	return count, nil
}
