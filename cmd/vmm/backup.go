package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	m_pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	t_pb "github.com/0xef53/kvmrun/api/services/tasks/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"

	empty "github.com/golang/protobuf/ptypes/empty"
	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
	grpc_codes "google.golang.org/grpc/codes"
)

var backupCommands = &cli.Command{
	Name:     "backup",
	Usage:    "manage backup tasks",
	HideHelp: true,
	Category: "Migration & Backup",
	Subcommands: []*cli.Command{
		cmdBackupStart,
		cmdBackupStatus,
		cmdBackupCancel,
	},
}

var cmdBackupStart = &cli.Command{
	Name:      "start",
	Usage:     "start backup of a virtual machine or disk",
	ArgsUsage: "VMNAME [DISKNAME] TARGET",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "watch", Aliases: []string{"w"}, Usage: "watch the process"},
		&cli.BoolFlag{Name: "incremental", Aliases: []string{"inc", "i"}, Usage: "only copy data described by a dirty bitmap (if exists)"},
		&cli.BoolFlag{Name: "clear-bitmap", Usage: "clear/reset a dirty bitmap (if exists) before starting"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, startBackupProcess)
	},
}

func startBackupProcess(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	var err error

	if len(c.Args().Tail()) >= 2 {
		req := m_pb.StartDiskBackupRequest{
			Name:        vmname,
			DiskName:    c.Args().Tail()[0],
			Target:      c.Args().Tail()[1],
			Incremental: c.Bool("incremental"),
			ClearBitmap: c.Bool("clear-bitmap"),
		}

		_, err = m_pb.NewMachineServiceClient(conn).StartDiskBackupProcess(ctx, &req)
	} else {
		err = fmt.Errorf("method is not implemented")
	}

	if err != nil {
		return err
	}

	if c.Bool("watch") {
		return showBackupProcessStatus(ctx, vmname, c, conn)
	} else {
		fmt.Println("Process has started and will continue in the background")
		fmt.Println("Use this command to see the progress:")
		fmt.Println("=> vmm backup status", vmname)
	}

	return nil
}

var cmdBackupStatus = &cli.Command{
	Name:      "status",
	Usage:     "check the progress of a backup",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, showBackupProcessStatus)
	},
}

func showBackupProcessStatus(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	client := t_pb.NewTaskServiceClient(conn)

	tasks := make(map[string]string)
	terrs := make(map[string]error)

	if resp, err := client.ListKeys(ctx, new(empty.Empty)); err == nil {
		for _, key := range resp.Tasks {
			if strings.HasPrefix(key, "backup:"+vmname+":") {
				tasks[key] = strings.Split(key, ":")[2]
			}
		}

		if c.Bool("json") {
			if len(tasks) == 0 {
				fmt.Println("[]")
				return nil
			}

			ss := make([]*pb_types.TaskInfo, 0, len(tasks))

			for key := range tasks {
				resp, err := client.Get(ctx, &t_pb.GetTaskRequest{Key: key})
				if err != nil {
					if IsGRPCError(err, grpc_codes.NotFound) {
						err = fmt.Errorf("unexpected: task not found: %s", key)
					}
					return err
				}
				ss = append(ss, resp.Task)
			}

			b, err := json.MarshalIndent(ss, "", "    ")
			if err != nil {
				return err
			}

			fmt.Printf("%s\n", b)

			return nil
		}

		if len(tasks) == 0 {
			fmt.Println("No one backup task found for", vmname)
			return nil
		}
	} else {
		return err
	}

	collect := func(ctx context.Context, update func(name string, p int)) error {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			if len(tasks) == 0 {
				break
			}

			for key, barname := range tasks {
				var progress int32

				resp, err := client.Get(ctx, &t_pb.GetTaskRequest{Key: key})
				if err != nil {
					if IsGRPCError(err, grpc_codes.NotFound) {
						err = fmt.Errorf("unexpected: task not found: %s", key)
					}
					terrs[key] = err
					delete(tasks, key)
					progress = -1
				} else {
					progress = resp.Task.GetProgress()

					switch resp.Task.State {
					case pb_types.TaskInfo_COMPLETED:
						delete(tasks, key)
					case pb_types.TaskInfo_FAILED:
						delete(tasks, key)
						terrs[key] = fmt.Errorf(resp.Task.StateDesc)
						progress = -1
					}
				}

				update(barname, int(progress))

				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
				}

			}
		}

		return nil
	}

	barNames := make([]string, 0, len(tasks))
	for _, name := range tasks {
		barNames = append(barNames, name)
	}

	progressBar := NewProgressBar(collect, barNames...)

	progressBar.Show()

	switch len(terrs) {
	case 0:
		fmt.Println("Successfully completed")
	case 1:
		for _, err := range terrs {
			return err
		}
	default:
		errmsg := "Some tasks completed with errors:\n"
		for tid, err := range terrs {
			errmsg += fmt.Sprintf("  * %s: %s\n", tid, err)
		}
		return fmt.Errorf(errmsg)
	}

	return nil
}

var cmdBackupCancel = &cli.Command{
	Name:      "cancel",
	Usage:     "cancel a running backup process",
	ArgsUsage: "VMNAME [DISKNAME]",
	HideHelp:  true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, cancelBackupProcess)
	},
}

func cancelBackupProcess(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := t_pb.CancelTaskRequest{}

	if len(c.Args().Tail()) >= 1 {
		req.Key = "backup:" + vmname + ":" + c.Args().Tail()[0] + ":"
	} else {
		return fmt.Errorf("method is not implemented")
	}

	_, err := t_pb.NewTaskServiceClient(conn).Cancel(ctx, &req)

	return err
}
