package system

import (
	"context"
	"fmt"
	"syscall"
	"time"

	pb "github.com/0xef53/kvmrun/api/services/system/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/services"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	grpc "google.golang.org/grpc"
)

var _ pb.SystemServiceServer = &ServiceServer{}

func init() {
	services.Register(&ServiceServer{})
}

type ServiceServer struct {
	*services.ServiceServer
}

func (s *ServiceServer) Init(inner *services.ServiceServer) {
	s.ServiceServer = inner
}

func (s *ServiceServer) Name() string {
	return fmt.Sprintf("%T", s)
}

func (s *ServiceServer) Register(server *grpc.Server) {
	pb.RegisterSystemServiceServer(server, s)
}

func (s *ServiceServer) RegisterQemuInstance(ctx context.Context, req *pb.RegisterQemuInstanceRequest) (*pb.RegisterQemuInstanceResponse, error) {
	t := NewQemuInstanceRegistrationTask(req, s.ServiceServer)

	key, err := s.StartTask(ctx, t, nil)
	if err != nil {
		return nil, err
	}

	return &pb.RegisterQemuInstanceResponse{TaskKey: key}, nil
}

func (s *ServiceServer) UnregisterQemuInstance(ctx context.Context, req *pb.UnregisterQemuInstanceRequest) (*empty.Empty, error) {
	s.Mon.CloseMonitor(req.Name)

	return new(empty.Empty), nil
}

func (s *ServiceServer) StopQemuInstance(ctx context.Context, req *pb.StopQemuInstanceRequest) (*empty.Empty, error) {
	t := NewQemuInstanceShutdownTask(req, s.ServiceServer)

	key, err := s.StartTask(ctx, t, nil)
	if err != nil {
		return nil, err
	}

	s.Tasks.Wait(key)

	return new(empty.Empty), nil
}

func (s *ServiceServer) StartIncomingMachine(ctx context.Context, req *pb.StartIncomingMachineRequest) (*pb.StartIncomingMachineResponse, error) {
	t := NewIncomingMachineTask(req, s.ServiceServer)

	resp := new(pb.StartIncomingMachineResponse)

	if _, err := s.StartTask(ctx, t, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *ServiceServer) StartDiskBackendProxy(ctx context.Context, req *pb.DiskBackendProxyRequest) (*empty.Empty, error) {
	vmconf, err := kvmrun.GetInstanceConf(req.Name)
	if err != nil {
		return nil, err
	}

	var gr errgroup.Group

	for _, proxy := range vmconf.GetProxyServers() {
		proxypath := proxy.Path

		gr.Go(func() error {
			log.WithField("machine", req.Name).Infof("Configuring proxy for disk %s", proxypath)

			be, err := kvmrun.NewDiskBackend(proxypath)
			if err != nil {
				return err
			}

			return s.ActivateDiskBackendProxy(req.Name, be.BaseName())
		})
	}

	if err := gr.Wait(); err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) StopDiskBackendProxy(ctx context.Context, req *pb.DiskBackendProxyRequest) (*empty.Empty, error) {
	units, err := s.SystemCtl.GetAllUnits(s.ProxyToUnit(req.Name, "*"))
	if err != nil {
		return nil, err
	}

	var gr errgroup.Group

	for _, unit := range units {
		unitname := unit.Name

		gr.Go(func() error {
			logFields := log.Fields{
				"machine": req.Name,
				"unit":    unitname,
			}

			log.WithFields(logFields).Infof("Deconfiguring proxy: %s", unitname)

			if err := s.SystemCtl.StopAndWait(unitname, 5*time.Second, nil); err != nil {
				log.WithFields(logFields).Errorf("Failed to shutdown proxy gracefully within 5 seconds: %s. Send SIGKILL", err)

				s.SystemCtl.KillBySIGKILL(unitname)
			}

			if err := s.SystemCtl.Disable(unitname); err != nil {
				log.WithFields(logFields).Errorf("Failed to deactivate proxy: %s", err)
			}

			return nil
		})
	}

	if err := gr.Wait(); err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) GracefulShutdown(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	log.Info("A graceful shutdown requested. SIGTEM will be sent to the kvmrund process")

	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	time.Sleep(3 * time.Second)

	return new(empty.Empty), nil
}

func (s *ServiceServer) GetAppConf(ctx context.Context, _ *empty.Empty) (*pb.GetAppConfResponse, error) {
	appConf := pb_types.AppConf{
		QemuRootdir: s.AppConf.Common.QemuRootDir,
	}

	return &pb.GetAppConfResponse{AppConf: &appConf}, nil
}
