package client

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_tasks "github.com/0xef53/kvmrun/api/services/tasks/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"

	bar "github.com/0xef53/kvmrun/client/progress_bar"

	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"

	cli "github.com/urfave/cli/v3"
)

func BackupProcessStart(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	var err error

	if len(c.Args().Tail()) >= 2 {
		req := pb_machines.StartDiskBackupRequest{
			Name:        vmname,
			DiskName:    c.Args().Tail()[0],
			Target:      c.Args().Tail()[1],
			Incremental: c.Bool("incremental"),
			ClearBitmap: c.Bool("clear-bitmap"),
		}

		_, err = grpcClient.Machines().StartDiskBackupProcess(ctx, &req)
	} else {
		err = fmt.Errorf("machine backup is not implemented")
	}

	if err != nil {
		return err
	}

	if c.Bool("watch") {
		return BackupProcessShowStatus(ctx, vmname, c, grpcClient)
	} else {
		fmt.Println("Process has started and will continue in the background")
		fmt.Println("Use this command to see the progress:")
		fmt.Println("vmm backup status", vmname)
	}

	return nil
}

func BackupProcessShowStatus(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	resp, err := grpcClient.Machines().Get(ctx, &pb_machines.GetRequest{Name: vmname})
	if err != nil {
		return err
	}

	if resp.Machine.Runtime == nil {
		fmt.Printf("No one backup task found: machine is not running: %s\n", vmname)

		return nil
	}

	disknames := make([]string, 0, len(resp.Machine.Runtime.Storage))
	labels := make([]string, 0, cap(disknames))

	/*
		TODO: need to use kvmrun.Disk to determine short disk name
	*/
	for _, d := range resp.Machine.Runtime.Storage {
		disknames = append(disknames, filepath.Base(d.Path))
		labels = append(labels, vmname+"/disk-backup/"+filepath.Base(d.Path))
	}

	req := pb_tasks.ListRequest{
		Keys: labels,
	}

	if resp, err := grpcClient.Tasks().List(ctx, &req); err == nil {
		if c.Bool("json") {
			b, err := json.MarshalIndent(resp.Tasks, "", "    ")
			if err != nil {
				return err
			}

			fmt.Printf("%s\n", b)

			return nil
		}

		if len(resp.Tasks) == 0 {
			fmt.Printf("No one backup task found for %s\n", vmname)

			return nil
		}
	} else {
		/*
			TODO: нужно проверить, что будет, если таски нет. Возможно тут стоит добавить
				вывод "{}" в таком случае, по аналогии с соседней ф-ией миграции.
		*/
		return fmt.Errorf("cannot request data: %w", err)
	}

	terrs := make(map[string]error)

	collect := func(ctx context.Context, update func(name string, p int)) error {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		// Tasks that completed for any reasons
		discontinued := make(map[string]struct{})

		for {
			for idx, label := range labels {
				if _, ok := discontinued[label]; ok {
					continue
				}

				var progress int

				resp, err := grpcClient.Tasks().Get(ctx, &pb_tasks.GetRequest{Key: label})
				if err != nil {
					if grpc_status.Code(err) != grpc_codes.NotFound {
						terrs[disknames[idx]] = err
					}

					discontinued[label] = struct{}{}

					progress = -1
				} else {
					progress = int(resp.Task.Progress)

					switch resp.Task.State {
					case pb_types.TaskInfo_COMPLETED:
						discontinued[label] = struct{}{}
					case pb_types.TaskInfo_FAILED:
						discontinued[label] = struct{}{}

						terrs[disknames[idx]] = fmt.Errorf("%s", resp.Task.StateDesc)

						progress = -1
					}
				}

				update(disknames[idx], progress)

				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
				}
			}

			if len(discontinued) == len(labels) {
				break
			}
		}

		return nil
	}

	progressBar := bar.NewProgressBar(collect, disknames...)

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

		for diskname, err := range terrs {
			errmsg += fmt.Sprintf("  * disk %s: %s\n", diskname, err)
		}

		return fmt.Errorf("%s", errmsg)
	}

	return nil
}

func BackupProcessCancel(ctx context.Context, vmname string, c *cli.Command, grpcClient *kvmrun_Interfaces) error {
	req := pb_tasks.CancelRequest{}

	if len(c.Args().Tail()) >= 1 {
		req.Key = vmname + "/disk-backup/" + c.Args().Tail()[0]
	} else {
		return fmt.Errorf("method is not implemented")
	}

	_, err := grpcClient.Tasks().Cancel(ctx, &req)

	return err
}
