package grpcserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc "google.golang.org/grpc"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Registration interface {
	Name() string
	Register(*grpc.Server)
}

type Server struct {
	conf     *ServerConf
	sockpath string

	grpcServer *grpc.Server
}

func NewServer(conf *ServerConf, services []Registration) *Server {
	// Unix Socket path
	var sockpath string

	if len(conf.BindSocket) > 0 {
		sockpath = conf.BindSocket
	} else {
		sockpath = filepath.Join("/run", fmt.Sprintf("%s_%d.sock", filepath.Base(os.Args[0]), os.Getpid()))
	}

	if runtime.GOOS == "linux" {
		sockpath = "@" + sockpath
	}

	// Composite Server
	srv := Server{
		conf:     conf,
		sockpath: sockpath,
	}

	// GRPC Server
	srv.grpcServer = grpc.NewServer(
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.TagBasedRequestFieldExtractor("log"))),
			grpc.UnaryServerInterceptor(unaryLogRequestInterceptor),
			grpc_logrus.UnaryServerInterceptor(log.NewEntry(log.StandardLogger())),
		),
	)

	for _, s := range services {
		log.Info("Registering service: ", s.Name())

		s.Register(srv.grpcServer)
	}

	return &srv
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	grpcLL, err := s.conf.Listeners()
	if err != nil {
		return err
	}

	// Default GRPC on Unix Socket
	if l, err := net.Listen("unix", s.sockpath); err == nil {
		defer l.Close()

		grpcLL = append(grpcLL, l)
	} else {
		return err
	}

	group, ctx := errgroup.WithContext(ctx)

	idleConnsClosed := make(chan struct{})

	go func() {
		<-ctx.Done()

		s.grpcServer.GracefulStop()

		close(idleConnsClosed)
	}()

	for _, l := range grpcLL {
		listener := l
		group.Go(func() error {
			log.WithFields(log.Fields{"addr": listener.Addr().String()}).Info("Starting GRPC server")

			if err := s.grpcServer.Serve(listener); err != nil {
				// Error starting or closing listener
				return err
			}

			log.WithFields(log.Fields{"addr": listener.Addr().String()}).Info("GRPC server stopped")

			return nil
		})
	}

	<-idleConnsClosed

	if err := group.Wait(); err != nil {
		return fmt.Errorf("GRPC Server error: %s", err)
	}

	return nil
}
