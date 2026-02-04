package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xef53/kvmrun/internal/appconf"
	"github.com/0xef53/kvmrun/server"

	"github.com/0xef53/kvmrun/services"
	"github.com/0xef53/kvmrun/services/interceptors"

	_ "github.com/0xef53/kvmrun/services/cloudinit"
	_ "github.com/0xef53/kvmrun/services/hardware"
	_ "github.com/0xef53/kvmrun/services/machines"
	_ "github.com/0xef53/kvmrun/services/network"
	_ "github.com/0xef53/kvmrun/services/system"
	_ "github.com/0xef53/kvmrun/services/tasks"

	grpcserver "github.com/0xef53/go-grpc/composite"

	"google.golang.org/grpc"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true,
	})
}

func main() {
	app := new(cli.Command)

	app.Usage = "GRPC/REST interface for managing virtual machines"
	app.Action = run

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Usage:   "path to the configuration file",
			Sources: cli.EnvVars("KVMRUND_CONFIG"),
			Value:   "/etc/kvmrun/kvmrun.ini",
		},
		&cli.BoolFlag{
			Name:    "debug",
			Usage:   "print debug information",
			Sources: cli.EnvVars("KVMRUND_DEBUG", "DEBUG"),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatalln(err)
	}
}

func run(ctx context.Context, c *cli.Command) error {
	if c.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	appConf, err := appconf.NewServerConfig(c.String("config"))
	if err != nil {
		return err
	}

	// This global cancel context is used by the graceful shutdown function
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Main request handlers
	var baseHandler *services.ServiceServer

	if h, err := server.NewServer(ctx, appConf); err == nil {
		if base, err := services.NewServiceServer(h); err == nil {
			baseHandler = base
		} else {
			return err
		}
	} else {
		return fmt.Errorf("pre-start error: %w", err)
	}

	// GRPC Server
	ui := []grpc.UnaryServerInterceptor{
		interceptors.MapErrorsUnaryServerInterceptor(),
	}

	srv, err := grpcserver.NewServer(&appConf.Server, appConf.Server.TLSConfig, ui, nil)
	if err != nil {
		return err
	}

	srv.SetServiceBuckets("kvmrun")

	// Register signal handler
	go func() {
		sigC := make(chan os.Signal, 1)

		signal.Notify(sigC, syscall.SIGTERM, syscall.SIGINT)
		defer signal.Stop(sigC)

		sig := <-sigC

		log.WithField("signal", sig).Info("Graceful shutdown initiated ...")

		for {
			n := len(baseHandler.Tasks.List())

			if n == 0 {
				baseHandler.Tasks.WaitAndClosePool()

				cancel()

				break
			}

			log.Warnf("Wait until all tasks finish (currently running: %d). Next attempt in 5 seconds", n)

			time.Sleep(5 * time.Second)
		}
	}()

	// Listen && serve
	srv.Start(ctx)

	if err := srv.Wait(); err != nil {
		return err
	}

	return nil
}
