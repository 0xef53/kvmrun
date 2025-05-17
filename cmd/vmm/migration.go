package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	m_pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	t_pb "github.com/0xef53/kvmrun/api/services/tasks/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"
	flag_types "github.com/0xef53/kvmrun/cmd/vmm/types"

	cli "github.com/urfave/cli/v2"
	grpc "google.golang.org/grpc"
	grpc_codes "google.golang.org/grpc/codes"
)

var migrationCommands = &cli.Command{
	Name:     "migration",
	Usage:    "manage migration tasks",
	HideHelp: true,
	Category: "Migration & Backup",
	Subcommands: []*cli.Command{
		cmdMigrationStart,
		cmdMigrationStatus,
		cmdMigrationCancel,
	},
}

var cmdMigrationStart = &cli.Command{
	Name:      "start",
	Usage:     "start migration of the virtual machine to another host",
	ArgsUsage: "VMNAME DSTSERVER",
	HideHelp:  true,
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "watch", Aliases: []string{"w"}, Usage: "watch the migration process"},
		&cli.BoolFlag{Name: "with-local-disks", Usage: "enable live storage migration for all local disks (conflicts with --with-disk)"},
		&cli.StringSliceFlag{Name: "with-disk", Usage: "enable live storage migration for specified disks (conflicts with --with-local-disks)"},
		&cli.GenericFlag{Name: "override-disk", Value: flag_types.NewStringMap(":"), DefaultText: "not set", Usage: "override disk path/name on the destination server"},
		&cli.BoolFlag{Name: "create-disks", Usage: "create logical volumes in the same group on the destination server"},
	},
	Action: func(c *cli.Context) error {
		return executeGRPC(c, startMigrationProcess)
	},
}

func startMigrationProcess(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	client := m_pb.NewMachineServiceClient(conn)

	req := m_pb.StartMigrationRequest{
		Name:      vmname,
		DstServer: c.Args().Tail()[0],
		Overrides: &pb_types.MigrationOverrides{
			Disks: c.Generic("override-disk").(*flag_types.StringMap).Value(),
		},
		CreateDisks: c.Bool("create-disks"),
		RemoveAfter: false,
	}

	switch {
	case c.Bool("with-local-disks"):
		resp, err := client.Get(ctx, &m_pb.GetMachineRequest{Name: vmname})
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
		for _, p := range c.StringSlice("with-disk") {
			req.Disks = append(req.Disks, p)
		}

	}

	if _, err := client.StartMigrationProcess(ctx, &req); err != nil {
		return err
	}

	if c.Bool("watch") {
		return showMigrationProcessStatus(ctx, vmname, c, conn)
	} else {
		fmt.Println("Process has started and will continue in the background")
		fmt.Println("Use this command to see the progress:")
		fmt.Println("=> vmm migration status", vmname)
	}

	return nil
}

var cmdMigrationStatus = &cli.Command{
	Name:      "status",
	Usage:     "check the progress of a migration",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, showMigrationProcessStatus)
	},
}

func showMigrationProcessStatus(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	client := t_pb.NewTaskServiceClient(conn)

	req := t_pb.GetTaskRequest{
		Key: "machine-migration:" + vmname + "::",
	}

	var barNames []string

	if resp, err := client.Get(ctx, &req); err == nil {
		details := resp.Task.GetMigration()
		if details == nil {
			return fmt.Errorf("unexpected: incorrect stat structure")
		}

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
		if IsGRPCError(err, grpc_codes.NotFound) {
			if c.Bool("json") {
				fmt.Println("{}")
			} else {
				fmt.Println("No migration process found for", vmname)
			}
			return nil
		}
		return err
	}

	collect := func(ctx context.Context, update func(name string, p int)) error {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			resp, err := client.Get(ctx, &req)

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
				if IsGRPCError(err, grpc_codes.NotFound) {
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

	progressBar := NewProgressBar(collect, barNames...)

	progressBar.Show()

	if progressBar.Err() == nil {
		fmt.Println("Successfully completed")
	} else {
		return progressBar.Err()
	}

	return nil
}

var cmdMigrationCancel = &cli.Command{
	Name:      "cancel",
	Usage:     "cancel a running migration process",
	ArgsUsage: "VMNAME",
	HideHelp:  true,
	Action: func(c *cli.Context) error {
		return executeGRPC(c, cancelMigrationProcess)
	},
}

func cancelMigrationProcess(ctx context.Context, vmname string, c *cli.Context, conn *grpc.ClientConn) error {
	req := t_pb.CancelTaskRequest{
		Key: "machine-migration:" + vmname + "::",
	}

	_, err := t_pb.NewTaskServiceClient(conn).Cancel(ctx, &req)

	return err
}
