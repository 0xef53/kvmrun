package client

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/0xef53/kvmrun/kvmrun"

	pb_mahines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	cli "github.com/urfave/cli/v3"
)

func MachineConsoleConnect(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	resp, err := grpcClient.Machines().Get(ctx, &pb_mahines.GetRequest{Name: vmname})
	if err != nil {
		return err
	}

	switch resp.Machine.State {
	case pb_types.MachineState_RUNNING, pb_types.MachineState_MIGRATING:
	default:
		return fmt.Errorf("machine is not running: %s", vmname)
	}

	socatBinary, err := exec.LookPath("socat")
	if err != nil {
		return err
	}

	sockPath := filepath.Join(kvmrun.QMPMONDIR, fmt.Sprint(vmname, ".virtcon"))

	socatOpts := []string{
		"/usr/bin/socat",
		"-lp", "kvmrun",
		"-L", fmt.Sprintf("%s.lock", sockPath),
		"-,raw,echo=0,escape=0x0f",
		fmt.Sprintf("UNIX-CONNECT:%s", sockPath),
	}

	fmt.Printf("Welcome to '%s' VM console\nPress Enter to start\nPress Ctrl-O to exit\n", vmname)

	if err := syscall.Exec("/usr/bin/socat", socatOpts, nil); err != nil {
		return fmt.Errorf("failed to run socat binary: %s", socatBinary)
	}

	return nil
}
