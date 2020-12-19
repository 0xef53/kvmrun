package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	qt "github.com/0xef53/kvmrun/pkg/qemu/types"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	qmp "github.com/0xef53/go-qmp/v2"
	log "github.com/sirupsen/logrus"
)

type DiskCopyingTaskOpts struct {
	VMName      string
	VMUid       int
	SrcDisk     *kvmrun.Disk
	DstDisk     *kvmrun.Disk
	SrcSize     uint64
	Incremental bool
	ClearBitmap bool
}

type DiskCopyingTask struct {
	task

	opts *DiskCopyingTaskOpts

	stat          *rpccommon.DiskCopyingTaskStat
	statPipe      chan StatUpdate
	terminateStat chan struct{}
	statCompleted chan struct{}
}

func (t *DiskCopyingTask) Stat() *rpccommon.DiskCopyingTaskStat {
	t.mu.Lock()
	defer t.mu.Unlock()

	x := *t.stat

	qemuJob := *t.stat.QemuJob
	x.QemuJob = &qemuJob

	return &x
}

func (t *DiskCopyingTask) updateStat(u *StatUpdate) {
	t.mu.Lock()
	defer t.mu.Unlock()

	printDebugStat := func(name string, st *rpccommon.StatInfo) {
		t.logger.WithFields(log.Fields{
			"object":      name,
			"total":       st.Total,
			"transferred": st.Transferred,
			"remaining":   st.Remaining,
			"percent":     st.Percent,
			"speed":       fmt.Sprintf("%dmbit/s", st.Speed),
		}).Debug()
	}

	switch u.Kind {
	case "status":
		t.stat.Status = u.NewStatus
	case "qemu_job":
		st := &rpccommon.StatInfo{}
		st.Total = u.Total
		st.Remaining = u.Remaining
		st.Transferred = u.Transferred
		st.Percent = uint(u.Transferred * 100 / u.Total)
		st.Speed = uint(((u.Transferred - t.stat.QemuJob.Transferred) * 8) / 1 >> (10 * 2)) // mbit/s
		t.stat.QemuJob = st
		printDebugStat(t.opts.SrcDisk.BaseName(), st)
	}

}

func (t *DiskCopyingTask) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel != nil {
		return fmt.Errorf("already running")
	}

	t.released = make(chan struct{})

	t.statPipe = make(chan StatUpdate, 10)
	t.terminateStat = make(chan struct{})
	t.statCompleted = make(chan struct{})

	t.stat = &rpccommon.DiskCopyingTaskStat{
		Status:  "starting",
		QemuJob: new(rpccommon.StatInfo),
	}

	t.err = nil

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	// Main process
	go func() {
		var err error

		t.logger.Info("Starting")

		defer func() {
			t.mu.Lock()
			t.cancel = nil
			t.err = err
			close(t.released)
			t.mu.Unlock()
		}()

		switch err = t.copy(ctx); {
		case err == nil:
			t.completed = true
			t.updateStat(&StatUpdate{Kind: "status", NewStatus: "completed"})
		case IsTaskInterruptedError(err):
			t.updateStat(&StatUpdate{Kind: "status", NewStatus: "interrupted"})
			t.logger.Warn("Interrupted by the CANCEL command")
		default:
			t.updateStat(&StatUpdate{Kind: "status", NewStatus: "failed"})
			t.logger.Errorf("Fatal error: %s", err)
		}

		//		// Wait until the stat collector is completed
		//		close(t.terminateStat)
		//		<-t.statCompleted

		if err == nil && t.completed {
			t.logger.Info("Successfully completed")
		}
	}()

	//	// Stat collecting
	//	go func() {
	//		defer close(t.statCompleted)
	//
	//		for {
	//			select {
	//			case <-t.terminateStat:
	//				return
	//			case x := <-t.statPipe:
	//				t.updateStat(&x)
	//			}
	//		}
	//	}()

	return nil
}

func (t *DiskCopyingTask) copy(ctx context.Context) error {
	t.logger.Debug("copy(): main process started")

	var success bool

	defer func() {
		if success {
			return
		}
		t.logger.Debug("copy(): something went wrong. Removing the scraps")
		t.cleanWhenInterrupted()
	}()

	t.updateStat(&StatUpdate{Kind: "status", NewStatus: "inprogress"})

	if t.opts.DstDisk.IsLocal() {
		if err := t.prepareChroot(t.opts.DstDisk.Path); err != nil {
			return err
		}
		defer func() {
			if err := os.Remove(filepath.Join(kvmrun.CHROOTDIR, t.opts.VMName, t.opts.DstDisk.Path)); err != nil {
				t.logger.Errorf("Failed to remove dstDisk from chroot: %s: %s", t.opts.DstDisk.Path, err)
			}
		}()
	}

	var ts time.Time

	t.logger.Debugf("copy(): running QMP command: drive-backup: src=%s, dst=%s", t.opts.SrcDisk.BaseName(), t.opts.DstDisk.Path)

	ts = time.Now()

	args := newQemuDriveBackupOpts(t.opts.SrcDisk.BaseName(), t.opts.DstDisk.Path)

	if t.opts.Incremental {
		bitmapArgs := qt.BlockDirtyBitmapOptions{
			Node: t.opts.SrcDisk.BaseName(),
			Name: "backup",
		}

		if t.opts.SrcDisk.HasBitmap {
			if t.opts.ClearBitmap {
				t.logger.Info("Mode: full backup (with bitmap reset)")
				t.logger.Debugf("copy(): running QMP transaction: block-dirty-bitmap-clear + drive-backup (src=%s, dst=%s", t.opts.SrcDisk.BaseName(), t.opts.DstDisk.Path)

				commands := []qmp.Command{
					qmp.Command{"block-dirty-bitmap-clear", &bitmapArgs},
					qmp.Command{"drive-backup", &args},
				}
				if err := t.mon.RunTransaction(t.opts.VMName, commands, nil); err != nil {
					return err
				}
			} else {
				t.logger.Info("Mode: incremental backup")
				t.logger.Debugf("copy(): running QMP command: drive-backup (src=%s, dst=%s)", t.opts.SrcDisk.BaseName(), t.opts.DstDisk.Path)

				args.Sync = "incremental"
				args.Bitmap = "backup"

				if err := t.mon.Run(t.opts.VMName, qmp.Command{"drive-backup", &args}, nil); err != nil {
					return err
				}
			}
		} else {
			t.logger.Info("Mode: full backup")
			t.logger.Debugf("copy(): running QMP transaction: block-dirty-bitmap-add + drive-backup (src=%s, dst=%s", t.opts.SrcDisk.BaseName(), t.opts.DstDisk.Path)

			commands := []qmp.Command{
				qmp.Command{"block-dirty-bitmap-add", bitmapArgs},
				qmp.Command{"drive-backup", &args},
			}
			if err := t.mon.RunTransaction(t.opts.VMName, commands, nil); err != nil {
				return err
			}
		}
	} else {
		t.logger.Info("Mode: full disk copying")

		if err := t.mon.Run(t.opts.VMName, qmp.Command{"drive-backup", &args}, nil); err != nil {
			return err
		}
	}

	// We start collecting statistics after the JOB_STATUS_CHANGE event
	// "JOB_STATUS_CHANGE", "data": {"status": "running", "id": "copy_st_93cb9996"}

	// and continue until the same event with the status == concluded
	// "JOB_STATUS_CHANGE", "data": {"status": "concluded", "id": "copy_st_93cb9996"}}
	// "Concluded" means that a task is done. Not necessarily successful.

	// Then if no errors detected we start waiting the BLOCK_JOB_COMPLETED event.

	errJobNotFound := errors.New("Job not found")

	jobID := "copy_" + t.opts.SrcDisk.BaseName()

	getJob := func() (*qt.BlockJobInfo, error) {
		jobs := make([]*qt.BlockJobInfo, 0, 1)
		if err := t.mon.Run(t.opts.VMName, qmp.Command{"query-block-jobs", nil}, &jobs); err != nil {
			return nil, err
		}
		for _, j := range jobs {
			if j.Device == jobID {
				return j, nil
			}
		}
		return nil, errJobNotFound
	}

	// Stat will be available after the job status changes to running
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := t.mon.WaitJobStatusChangeEvent(t.opts.VMName, timeoutCtx, jobID, "running", uint64(ts.Unix())); err != nil {
		return err
	}

	ts = time.Now()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return &TaskInterruptedError{}
		case <-ticker.C:
		}

		var completed bool

		job, err := getJob()
		switch {
		case err == nil:
		case err == errJobNotFound:
			if _, found, err := t.mon.FindBlockJobErrorEvent(t.opts.VMName, jobID, uint64(ts.Unix())); err == nil {
				if found {
					return fmt.Errorf("Errors detected during disk mirroring")
				}
			} else {
				return fmt.Errorf("FindBlockJobErrorEvent failed: %s", err)
			}
			if _, found, err := t.mon.FindBlockJobCompletedEvent(t.opts.VMName, jobID, uint64(ts.Unix())); err == nil {
				if !found {
					return fmt.Errorf("No completed event found")
				}
			} else {
				return fmt.Errorf("FindBlockJobCompletedEvent failed: %s", err)
			}
			completed = true
		default:
			return err
		}

		if completed {
			t.updateStat(&StatUpdate{
				Kind:        "qemu_job",
				Total:       t.opts.SrcSize,
				Remaining:   0,
				Transferred: t.opts.SrcSize,
			})
			break
		}

		t.updateStat(&StatUpdate{
			Kind:        "qemu_job",
			Total:       t.opts.SrcSize,
			Remaining:   job.Len - job.Offset,
			Transferred: t.opts.SrcSize - (job.Len - job.Offset),
		})
	}

	success = true

	t.logger.Debugf("copy(): completed")

	return nil
}

// This function partially duplicates the same name function from kvmrun/cmd/launcher.
func (t *DiskCopyingTask) prepareChroot(diskPath string) error {
	chrootDir := filepath.Join(kvmrun.CHROOTDIR, t.opts.VMName)

	if err := os.MkdirAll(filepath.Join(chrootDir, filepath.Dir(diskPath)), 0755); err != nil {
		return err
	}

	stat := syscall.Stat_t{}
	if err := syscall.Stat(diskPath, &stat); err != nil {
		return fmt.Errorf("stat %s: %s", diskPath, err)
	}

	if err := syscall.Mknod(filepath.Join(chrootDir, diskPath), syscall.S_IFBLK|uint32(os.FileMode(01600)), int(stat.Rdev)); err != nil {
		return fmt.Errorf("mknod %s: %s", diskPath, err)
	}

	return os.Chown(filepath.Join(chrootDir, diskPath), t.opts.VMUid, 0)
}

func (t *DiskCopyingTask) cleanWhenInterrupted() {
	cancelOpts := struct {
		Device string `json:"device"`
		Force  bool   `json:"force,omitempty"`
	}{
		Device: "copy_" + t.opts.SrcDisk.BaseName(),
		Force:  true,
	}

	if err := t.mon.Run(t.opts.VMName, qmp.Command{"block-job-cancel", &cancelOpts}, nil); err != nil {
		// Non-fatal error. Just printing
		t.logger.Errorf("Forced block-job-cancel failed: %s", err)
	}
}

func newQemuDriveBackupOpts(srcDiskName, dstDiskPath string) *qt.DriveBackupOptions {
	return &qt.DriveBackupOptions{
		JobID:  fmt.Sprintf("copy_%s", srcDiskName),
		Device: srcDiskName,
		Target: dstDiskPath,
		Sync:   "full",
		Mode:   "existing",
	}
}
