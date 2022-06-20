package machines

import (
	"context"
	"fmt"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"
	"github.com/0xef53/kvmrun/internal/randstring"
	"github.com/0xef53/kvmrun/kvmrun"

	log "github.com/sirupsen/logrus"
)

func (s *ServiceServer) ActivateVNC(ctx context.Context, req *pb.ActivateVNCRequest) (*pb.ActivateVNCResponse, error) {
	if len(req.Password) == 0 {
		req.Password = randstring.RandString(8)
	}

	requisites := pb_types.VNCRequisites{
		Password: req.Password,
	}

	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if vm.R == nil {
			return fmt.Errorf("not running: %s", req.Name)
		}

		requisites.Display = int32(vm.R.Uid())
		requisites.Port = int32(vm.R.Uid() + 5900)
		requisites.WSPort = int32(vm.R.Uid() + kvmrun.FIRST_WS_PORT)

		return vm.R.SetVNCPassword(req.Password)
	})

	if err != nil {
		return nil, err
	}

	return &pb.ActivateVNCResponse{Requisites: &requisites}, nil
}
