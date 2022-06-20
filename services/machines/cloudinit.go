package machines

import (
	"context"
	"os"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

func (s *ServiceServer) AttachCloudInitDrive(ctx context.Context, req *pb.AttachCloudInitRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if fi, err := os.Stat(req.Path); err == nil {
			if fi.IsDir() {
				return grpc_status.Errorf(grpc_codes.InvalidArgument, "not a file: %s", req.Path)
			}
		} else {
			if os.IsNotExist(err) {
				return grpc_status.Errorf(grpc_codes.InvalidArgument, "not found: %s", req.Path)
			}
		}

		if err := vm.C.SetCloudInitDrive(req.Path); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) DetachCloudInitDrive(ctx context.Context, req *pb.DetachCloudInitRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if err := vm.C.SetCloudInitDrive(""); err != nil {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) ChangeCloudInitDrive(ctx context.Context, req *pb.ChangeCloudInitRequest) (*empty.Empty, error) {
	return nil, grpc_status.Errorf(grpc_codes.Unimplemented, "method is not implemented")

	/*
		TODO: need to complete

		err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
			vm, err := s.GetMachine(req.Name)
			if err != nil {
				return err
			}

			_ = vm

			return nil
		})

		if err != nil {
			return nil, err
		}

		return new(empty.Empty), nil
	*/
}
