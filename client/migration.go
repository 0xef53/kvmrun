package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_tasks "github.com/0xef53/kvmrun/api/services/tasks/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"

	bar "github.com/0xef53/kvmrun/client/progress_bar"

	grpc_interfaces "github.com/0xef53/kvmrun/internal/grpc/interfaces"

	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"

	cli "github.com/urfave/cli/v3"
)

func MigrationProcessStart(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_machines.StartMigrationRequest{
		Name:        vmname,
		DstServer:   c.Args().Tail()[0],
		Overrides:   new(pb_types.MigrationOverrides),
		CreateDisks: c.Bool("create-disks"),
		RemoveAfter: false,
	}

	if c.IsSet("override-name") {
		req.Overrides.Name = c.String("override-name")
	}

	if c.Value("override-disk") != nil {
		if v, ok := c.Value("override-disk").(map[string]string); ok {
			req.Overrides.Disks = v
		}
	}

	if c.Value("override-net") != nil {
		if v, ok := c.Value("override-net").(map[string]string); ok {
			req.Overrides.NetIfaces = v
		}
	}

	switch {
	case c.Bool("with-local-disks"):
		resp, err := grpcClient.Machines().Get(ctx, &pb_machines.GetRequest{Name: vmname})
		if err != nil {
			return err
		}

		switch resp.Machine.State {
		case pb_types.MachineState_PAUSED, pb_types.MachineState_RUNNING:
		default:
			return fmt.Errorf("unable to start migration process: machine is not running: %s", vmname)
		}

		for _, d := range resp.Machine.Runtime.Storage {
			req.Disks = append(req.Disks, d.Path)
		}
	default:
		req.Disks = append(req.Disks, c.StringSlice("with-disk")...)
	}

	if _, err := grpcClient.Machines().StartMigrationProcess(ctx, &req); err != nil {
		return err
	}

	if c.Bool("watch") {
		return MigrationProcessShowStatus(ctx, vmname, c, grpcClient)
	} else {
		fmt.Println("Process has started and will continue in the background")
		fmt.Println("Use this command to see the progress:")
		fmt.Println("=> vmm migration status", vmname)
	}

	return nil
}

func MigrationProcessShowStatus(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_tasks.GetRequest{
		Key: vmname + "/migration",
	}

	var barNames []string

	if resp, err := grpcClient.Tasks().Get(ctx, &req); err == nil {
		details := resp.Task.GetMigration()

		if details == nil {
			return fmt.Errorf("unexpected: incorrect stat structure")
		}

		req.Key = resp.Task.TaskID

		if c.Bool("json") {
			b, err := json.MarshalIndent(details, "", "    ")
			if err != nil {
				return err
			}

			fmt.Printf("%s\n", b)

			return nil
		}

		for diskname := range details.Disks {
			barNames = append(barNames, diskname)
		}
		barNames = append(barNames, vmname)
	} else {
		if grpc_status.Code(err) == grpc_codes.NotFound {
			if c.Bool("json") {
				fmt.Println("{}")
			} else {
				fmt.Println("No migration process found for", vmname)
			}

			return nil
		}

		return fmt.Errorf("cannot request data: %w", err)
	}

	collect := func(ctx context.Context, update func(name string, p int)) error {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			resp, err := grpcClient.Tasks().Get(ctx, &req)

			if err == nil {
				details := resp.Task.GetMigration()

				if details != nil {
					update(vmname, int(details.Qemu.Progress))

					for diskname := range details.Disks {
						update(diskname, int(details.Disks[diskname].Progress))
					}
				}

				if resp.Task.State == pb_types.TaskInfo_COMPLETED {
					break
				}

				if resp.Task.State == pb_types.TaskInfo_FAILED {
					err = fmt.Errorf("%s", resp.Task.StateDesc)
				}
			}

			if err != nil {
				if grpc_status.Code(err) == grpc_codes.NotFound {
					err = fmt.Errorf("unexpected: task not found")
				}

				for _, name := range barNames {
					update(name, -1)
				}

				return err
			}

			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
			}
		}

		return nil
	}

	progressBar := bar.NewProgressBar(collect, barNames...)

	progressBar.Show()

	if progressBar.Err() == nil {
		fmt.Println("Successfully completed")
	} else {
		return progressBar.Err()
	}

	return nil
}

func MigrationProcessCancel(ctx context.Context, vmname string, c *cli.Command, grpcClient *grpc_interfaces.Kvmrun) error {
	req := pb_tasks.CancelRequest{
		Key: vmname + "/migration",
	}

	_, err := grpcClient.Tasks().Cancel(ctx, &req)

	return err
}
