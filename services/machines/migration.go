package machines

import (
	"context"

	"github.com/0xef53/kvmrun/kvmrun"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"

	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

func (s *ServiceServer) StartMigrationProcess(ctx context.Context, req *pb.StartMigrationRequest) (*pb.StartMigrationResponse, error) {
	vm, err := s.GetMachine(req.Name)
	if err != nil {
		return nil, err
	}

	vmstate, err := s.GetMachineStatus(vm)
	if err != nil {
		return nil, err
	}

	switch vmstate {
	case kvmrun.StatePaused, kvmrun.StateRunning:
	default:
		return nil, grpc_status.Errorf(grpc_codes.FailedPrecondition, "unable to start while machine is in the '%s' state", pb_types.MachineState_name[int32(vmstate)])
	}

	t := NewMachineMigrationTask(req, s.ServiceServer, vm)

	key, err := s.StartTask(ctx, t, nil)
	if err != nil {
		return nil, err
	}

	return &pb.StartMigrationResponse{TaskKey: key}, nil
}
