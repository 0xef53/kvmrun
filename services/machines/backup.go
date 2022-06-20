package machines

import (
	"context"
	"fmt"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"
	"github.com/0xef53/kvmrun/kvmrun"

	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

func (s *ServiceServer) StartDiskBackupProcess(ctx context.Context, req *pb.StartDiskBackupRequest) (*pb.StartBackupResponse, error) {
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

	// Just to convert req.DiskName to the short notation
	if vm.R != nil {
		d := vm.R.GetDisks().Get(req.DiskName)
		if d == nil {
			return nil, fmt.Errorf("not attached to the running QEMU instance: %s", req.DiskName)
		}
		req.DiskName = d.BaseName()
	} else {
		return nil, fmt.Errorf("unexpected: a running QEMU instance is not found")
	}

	t := NewDiskBackupTask(req, s.ServiceServer, vm)

	key, err := s.StartTask(ctx, t, nil)
	if err != nil {
		return nil, err
	}

	return &pb.StartBackupResponse{TaskKey: key}, nil
}
