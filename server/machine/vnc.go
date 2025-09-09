package machine

import (
	"context"
	"fmt"

	"github.com/0xef53/kvmrun/internal/randstring"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

type VNCRequisites struct {
	Password string `json:"password"`
	Display  int    `json:"display"`
	Port     int    `json:"port"`
	WSPort   int    `json:"ws_port"`
}

func (s *Server) VNCActivate(ctx context.Context, vmname, password string) (*VNCRequisites, error) {
	if len(password) == 0 {
		password = randstring.RandString(8)
	}

	requisites := VNCRequisites{
		Password: password,
	}

	err := s.TaskRunFunc(ctx, server.NoBlockOperations(vmname), true, nil, func(l *log.Entry) error {
		vm, err := s.MachineGet(vmname, true)
		if err != nil {
			return err
		}

		if vm.R == nil {
			return fmt.Errorf("not running: %s", vmname)
		}

		requisites.Display = vm.R.UID()
		requisites.Port = vm.R.UID() + 5900
		requisites.WSPort = vm.R.UID() + kvmrun.FIRST_WS_PORT

		return vm.R.VNCSetPassword(password)
	})

	if err != nil {
		return nil, fmt.Errorf("cannot activate VNC: %w", err)
	}

	return &requisites, nil
}
