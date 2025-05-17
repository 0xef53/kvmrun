package machines

import (
	"context"
	"errors"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	"github.com/0xef53/kvmrun/internal/pci"
	"github.com/0xef53/kvmrun/kvmrun"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

func (s *ServiceServer) AttachHostPCIDevice(ctx context.Context, req *pb.AttachHostPCIDeviceRequest) (*empty.Empty, error) {
	hpci, err := kvmrun.NewHostPCI(req.Addr)
	if err != nil {
		return nil, err
	}

	if req.StrictMode {
		if _, err := pci.LookupDevice(hpci.Addr); err != nil {
			if errors.Is(err, pci.ErrDeviceNotFound) {
				return nil, grpc_status.Error(grpc_codes.NotFound, err.Error())
			}
			return nil, err
		}
	}

	hpci.Multifunction = req.Multifunction
	hpci.PrimaryGPU = req.PrimaryGPU

	err = s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.AppendHostPCI(*hpci); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
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
	hpci, err := kvmrun.NewHostPCI(req.Addr)
	if err != nil {
		return nil, err
	}

	err = s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.RemoveHostPCI(hpci.BackendAddr.String()); err != nil && !kvmrun.IsNotConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) SetHostPCIMultifunctionOption(ctx context.Context, req *pb.SetHostPCIMultifunctionOptionRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.SetHostPCIMultifunctionOption(req.Addr, req.Enabled); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) SetHostPCIPrimaryGPUOption(ctx context.Context, req *pb.SetHostPCIPrimaryGPUOptionRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.SetHostPCIPrimaryGPUOption(req.Addr, req.Enabled); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
