package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"
	"github.com/0xef53/kvmrun/kvmrun"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
)

var cmdConsole = &cli.Command{
	Name:      "console",
	Usage:     "connect to a virtual machine console",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Category:  "Configuration",
	Action: func(c *cli.Context) error {
		return executeGRPC(c, getConsole)
	},
}

func getConsole(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	resp, err := pb.NewMachineServiceClient(conn).Get(ctx, &pb.GetMachineRequest{Name: vmname})
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
		"-lp", c.App.Name,
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
