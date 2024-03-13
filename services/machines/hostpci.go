package machines

import (
	"context"
	"os"
	"path/filepath"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	"github.com/0xef53/kvmrun/kvmrun"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

func (s *ServiceServer) AttachHostPCIDevice(ctx context.Context, req *pb.AttachHostPCIDeviceRequest) (*empty.Empty, error) {
	pcidev, err := kvmrun.NewHostPCI(req.Addr)
	if err != nil {
		return nil, err
	}

	if req.StrictMode {
		if _, err := os.Stat(filepath.Join(pcidev.Backend.FullPath(), "config")); err != nil {
			return nil, grpc_status.Errorf(grpc_codes.NotFound, "PCI device not found: %s", pcidev.Addr)
		}
	}

	pcidev.Multifunction = req.Multifunction
	pcidev.PrimaryGPU = req.PrimaryGPU

	err = s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.AppendHostPCI(*pcidev); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) DetachHostPCIDevice(ctx context.Context, req *pb.DetachHostPCIDeviceRequest) (*empty.Empty, error) {
	// Validate PCI address format
	pcidev, err := kvmrun.NewHostPCI(req.Addr)
	if err != nil {
		return nil, err
	}

	err = s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.RemoveHostPCI(pcidev.Backend.String()); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
