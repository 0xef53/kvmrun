package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	rpcclient "github.com/0xef53/kvmrun/pkg/rpc/client"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	"github.com/0xef53/cli"

	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uiprogress/util/strutil"
)

var cmdAttachDisk = cli.Command{
	Name:      "attach-disk",
	Usage:     "attach a new disk device",
	ArgsUsage: "VMNAME DISK",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "driver", Value: "virtio-blk-pci", Usage: "new disk device driver name"},
		cli.IntFlag{Name: "iops-rd", PlaceHolder: "0", Usage: "read I/O operations limit per second"},
		cli.IntFlag{Name: "iops-wr", PlaceHolder: "0", Usage: "write I/O operations limit per second"},
		cli.IntFlag{Name: "index", Value: -1, PlaceHolder: "UINT", Usage: "index number (0 -- is bootable device)"},
	},
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, attachDisk))
	},
}

func attachDisk(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &rpccommon.DiskParams{
			Path:   c.Args().Tail()[0],
			Driver: c.String("driver"),
			IopsRd: c.Int("iops-rd"),
			IopsWr: c.Int("iops-wr"),
			Index:  c.Int("index"),
		},
	}

	if err := client.Request("RPC.AttachDisk", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}

var cmdDetachDisk = cli.Command{
	Name:      "detach-disk",
	Usage:     "detach an existing disk device",
	ArgsUsage: "VMNAME DISK",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, detachDisk))
	},
}

func detachDisk(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &rpccommon.DiskParams{
			Path: c.Args().Tail()[0],
		},
	}

	if err := client.Request("RPC.DetachDisk", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}

var cmdUpdateDisk = cli.Command{
	Name:      "update-disk",
	Usage:     "update paramaters of a disk device",
	ArgsUsage: "VMNAME DISK",
	Flags: []cli.Flag{
		cli.IntFlag{Name: "iops-rd", Value: -1, PlaceHolder: "UINT", Usage: "read I/O operations limit per second"},
		cli.IntFlag{Name: "iops-wr", Value: -1, PlaceHolder: "UINT", Usage: "write I/O operations limit per second"},
		cli.BoolFlag{Name: "remove-bitmap", Usage: "stop write tracking and remove the dirty bitmap (if exists)"},
	},
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, updateDisk))
	},
}

func updateDisk(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	params := rpccommon.DiskParams{
		Path:   c.Args().Tail()[0],
		IopsRd: c.Int("iops-rd"),
		IopsWr: c.Int("iops-wr"),
	}

	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &params,
	}

	if params.IopsRd != -1 || params.IopsWr != -1 {
		if err := client.Request("RPC.SetDiskIops", &req, nil); err != nil {
			return append(errors, err)
		}
	}

	if c.Bool("remove-bitmap") {
		if err := client.Request("RPC.RemoveDiskBitmap", &req, nil); err != nil {
			return append(errors, err)
		}
	}

	return errors
}

var cmdResizeDisk = cli.Command{
	Name:      "resize-disk",
	Usage:     "resize a disk device",
	ArgsUsage: "VMNAME DISK",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, resizeDisk))
	},
}

func resizeDisk(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &rpccommon.DiskParams{
			Path: c.Args().Tail()[0],
		},
	}

	if err := client.Request("RPC.ResizeDisk", &req, nil); err != nil {
		return append(errors, err)
	}

	return errors
}

var cmdCopyDisk = cli.Command{
	Name:      "copy-disk",
	Usage:     "copy a disk content to another destination disk",
	ArgsUsage: "VMNAME SRCDISK DSTDISK",
	Flags: []cli.Flag{
		cli.BoolFlag{Name: "watch,w", Usage: "watch the operation process"},
		cli.BoolFlag{Name: "incremental,inc,i", Usage: "only copy data described by a dirty bitmap (if exists)"},
		cli.BoolFlag{Name: "clear-bitmap", Usage: "clear/reset a dirty bitmap (if exists) before starting"},
	},
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, copyDisk))
	},
}

func copyDisk(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.InstanceRequest{
		Name: vmname,
		Live: live,
		Data: &rpccommon.DiskCopyingParams{
			SrcName:     c.Args().Tail()[0],
			TargetURI:   c.Args().Tail()[1],
			Incremental: c.Bool("incremental"),
			ClearBitmap: c.Bool("clear-bitmap"),
		},
	}

	if err := client.Request("RPC.StartDiskCopyingProcess", &req, nil); err != nil {
		return append(errors, err)
	}

	if c.Bool("watch") {
		return diskJobStatus(vmname, live, c, client)
	} else {
		fmt.Println("Process started")
		fmt.Println("Note: command 'disk-job-status' shows the progress")
	}

	return errors
}

var cmdDiskJobCancel = cli.Command{
	Name:      "disk-job-cancel",
	Usage:     "cancel a running disk job",
	ArgsUsage: "VMNAME JOB_ID",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, diskJobCancel))
	},
}

func diskJobCancel(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.DiskJobIDRequest{
		JobID: c.Args().Tail()[0],
	}

	if err := client.Request("RPC.CancelDiskJobProcess", &req, nil); err != nil {
		return append(errors, err)
	}

	fmt.Println("OK, cancelled")

	return errors
}

var cmdDiskJobStatus = cli.Command{
	Name:      "disk-job-status",
	Usage:     "check a progress of a running job or a final result",
	ArgsUsage: "VMNAME JOB_ID",
	Action: func(c *cli.Context) {
		os.Exit(executeRPC(c, diskJobStatus))
	},
}

func diskJobStatus(vmname string, live bool, c *cli.Context, client *rpcclient.UnixClient) (errors []error) {
	req := rpccommon.DiskJobIDRequest{
		JobID: c.Args().Tail()[0],
	}

	st := rpccommon.DiskJobStat{}

	if err := client.Request("RPC.GetDiskJobStat", &req, &st); err != nil {
		return append(errors, err)
	}

	// Just print and exit
	if c.GlobalBool("json") {
		jb, err := json.MarshalIndent(st, "", "    ")
		if err != nil {
			return append(errors, err)
		}

		fmt.Printf("%s\n", string(jb))

		return errors
	}

	switch st.Status {
	case "completed":
		fmt.Println("Successfully completed")
		return errors
	case "none":
		return append(errors, fmt.Errorf("Process is not running"))
	case "failed":
		return append(errors, fmt.Errorf("Process failed: %s", st.Desc))
	case "interrupted":
		return append(errors, fmt.Errorf("Process is interrupted"))
	}

	completed := make(chan struct{})
	barPipe := make(chan *rpccommon.StatInfo, 10)

	// This function prints a progress bar for disk
	process := func(name string, pipe <-chan *rpccommon.StatInfo) {
		bar := uiprogress.AddBar(100).AppendCompleted()
		bar.Width = 50

		var status string

		bar.PrependFunc(func(b *uiprogress.Bar) string {
			return strutil.Resize(fmt.Sprintf("%s: %*s", name, (32-len(name)), status), 35)
		})

		for {
			select {
			case <-completed:
				bar.Set(100)
				return
			case x := <-pipe:
				switch {
				case x.Percent == 0:
					status = "waiting"
				case x.Percent == 100:
					status = "completed"
				default:
					status = "syncing"
				}
				bar.Set(int(x.Percent))
			}
		}
	}

	uiprogress.Start()

	barPipe = make(chan *rpccommon.StatInfo)
	go process(c.Args().Tail()[0], barPipe)
	barPipe <- st.QemuJob

	// Watch the progress ...
loop:
	for {
		st := rpccommon.DiskJobStat{}

		if err := client.Request("RPC.GetDiskJobStat", &req, &st); err != nil {
			return append(errors, err)
		}

		switch st.Status {
		case "completed", "none":
			close(completed)
			// workaround to make sure that the progress bar
			// will have enough time to show 100%
			time.Sleep(1 * time.Second)
			break loop
		case "inprogress":
			barPipe <- st.QemuJob
			time.Sleep(1 * time.Second)
		case "failed", "interrupted":
			break loop
		}
	}

	uiprogress.Stop()
	fmt.Println()

	// Print results
	if err := client.Request("RPC.GetDiskJobStat", &req, &st); err != nil {
		return append(errors, err)
	}

	switch st.Status {
	case "completed", "none":
		fmt.Println("Successfully completed")
	case "failed":
		errors = append(errors, fmt.Errorf("Process failed: %s", st.Desc))
	case "interrupted":
		errors = append(errors, fmt.Errorf("Process is interrupted"))
	}

	return errors
}
