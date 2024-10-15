package machines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	qemu_types "github.com/0xef53/kvmrun/internal/qemu/types"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/internal/types"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/services"

	qmp "github.com/0xef53/go-qmp/v2"
)

type DiskBackupTask struct {
	*task.GenericTask
	*services.ServiceServer

	mu sync.Mutex

	vm  *kvmrun.Machine
	req *pb.StartDiskBackupRequest

	srcSize uint64
	srcDisk *kvmrun.Disk
	dstDisk *kvmrun.Disk

	details  *types.DiskBackupDetails
	statfile string
}

func NewDiskBackupTask(req *pb.StartDiskBackupRequest, ss *services.ServiceServer, vm *kvmrun.Machine) *DiskBackupTask {
	return &DiskBackupTask{
		GenericTask:   new(task.GenericTask),
		ServiceServer: ss,
		req:           req,
		vm:            vm,
	}
}

func (t *DiskBackupTask) GetNS() string { return "backup" }

func (t *DiskBackupTask) GetKey() string { return t.req.Name + ":" + t.req.DiskName + ":" }

func (t *DiskBackupTask) BeforeStart(_ interface{}) error {
	if t.vm.R == nil {
		return fmt.Errorf("not running: %s", t.req.Name)
	}

	srcDisk := t.vm.R.GetDisks().Get(t.req.DiskName)
	if srcDisk == nil {
		return fmt.Errorf("not attached to the running QEMU instance: %s", t.req.DiskName)
	}

	if t.vm.R.GetDisks().Exists(t.req.Target) {
		return fmt.Errorf("target device is connected to the current machine: %s", t.req.Target)
	}

	var srcSize uint64

	if srcDisk.IsLocal() {
		s, err := srcDisk.Backend.Size()
		if err != nil {
			return err
		}
		srcSize = s
	} else {
		srcSize = srcDisk.QemuVirtualSize
	}

	dstDisk, err := kvmrun.NewDisk(t.req.Target)
	if err != nil {
		return err
	}

	if ok, err := dstDisk.IsAvailable(); !ok {
		return err
	}

	if dstDisk.IsLocal() {
		dstSize, err := dstDisk.Backend.Size()
		if err != nil {
			return err
		}

		if dstSize < srcSize {
			return fmt.Errorf("target device size is smaller than the source device (%d < %d)", dstSize, srcSize)
		}
	}

	t.srcSize = srcSize
	t.srcDisk = srcDisk
	t.dstDisk = dstDisk

	t.details = &types.DiskBackupDetails{
		Disk: new(types.DataTransferStat),
	}

	t.statfile = filepath.Join(kvmrun.CHROOTDIR, t.req.Name, ".tasks", t.GetNS()+":"+t.GetKey())

	if err := os.Remove(t.statfile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (t *DiskBackupTask) Main() error {
	var err error

	saveStatToFile := func(st *task.TaskStat) error {
		b, err := json.MarshalIndent(st, "", "    ")
		if err != nil {
			return err
		}
		return os.WriteFile(t.statfile, b, 0644)
	}

	defer func() {
		st := t.Stat()

		if err == nil {
			st.State = task.StateCompleted
		} else {
			st.State = task.StateFailed
			st.StateDesc = err.Error()
		}

		if err := saveStatToFile(st); err != nil {
			t.Logger.Errorf("Failed to save the task state to a file: %s", err)
		}
	}()

	err = t.copy()

	return err
}

func (t *DiskBackupTask) OnFailure() error {
	opts := struct {
		Device string `json:"device"`
		Force  bool   `json:"force,omitempty"`
	}{
		Device: "copy_" + t.srcDisk.BaseName(),
		Force:  true,
	}

	if err := t.Mon.Run(t.req.Name, qmp.Command{"block-job-cancel", &opts}, nil); err != nil {
		// Non-fatal error. Just printing
		t.Logger.Errorf("OnFailureHook: forced block-job-cancel failed: %s", err)
	}

	return nil
}

type diskBackupTaskStatUpdate struct {
	Total       uint64
	Remaining   uint64
	Transferred uint64
	Speed       int32
}

func (t *DiskBackupTask) updateStat(u *diskBackupTaskStatUpdate) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if u.Total == 0 {
		return
	}

	t.details.Disk = &types.DataTransferStat{
		Total:       u.Total,
		Remaining:   u.Remaining,
		Transferred: u.Transferred,
		Progress:    int32(u.Transferred * 100 / u.Total),
		Speed:       u.Speed,
	}

	t.SetProgress(t.details.Disk.Progress)
}

func (t *DiskBackupTask) copy() error {
	t.Logger.Debug("copy(): main process started")

	if t.dstDisk.IsLocal() {
		defer os.Remove(filepath.Join(kvmrun.CHROOTDIR, t.req.Name, t.dstDisk.Path))

		if err := t.mapDeviceToChroot(t.dstDisk.Path); err != nil {
			return err
		}
	}

	t.Logger.Debugf("copy(): run QMP command: drive-backup: src=%s, dst=%s", t.srcDisk.BaseName(), t.dstDisk.Path)

	ts := time.Now()

	backupArgs := qemu_types.DriveBackupOptions{
		JobID:  fmt.Sprintf("copy_%s", t.srcDisk.BaseName()),
		Device: t.srcDisk.BaseName(),
		Target: t.dstDisk.Path,
		Sync:   "full",
		Mode:   "existing",
	}

	if t.req.Incremental {
		bitmapArgs := qemu_types.BlockDirtyBitmapOptions{
			Node: t.srcDisk.BaseName(),
			Name: "backup",
		}

		if t.srcDisk.HasBitmap {
			if t.req.ClearBitmap {
				t.Logger.Info("Mode: full backup (with bitmap reset)")
				t.Logger.Debugf("copy(): run QMP transaction: block-dirty-bitmap-clear + drive-backup (src=%s, dst=%s", t.srcDisk.BaseName(), t.dstDisk.Path)

				commands := []qmp.Command{
					{"block-dirty-bitmap-clear", &bitmapArgs},
					{"drive-backup", &backupArgs},
				}
				if err := t.Mon.RunTransaction(t.req.Name, commands, nil); err != nil {
					return err
				}
			} else {
				t.Logger.Info("Mode: incremental backup")
				t.Logger.Debugf("copy(): run QMP command: drive-backup (src=%s, dst=%s)", t.srcDisk.BaseName(), t.dstDisk.Path)

				backupArgs.Sync = "incremental"
				backupArgs.Bitmap = "backup"

				if err := t.Mon.Run(t.req.Name, qmp.Command{"drive-backup", &backupArgs}, nil); err != nil {
					return err
				}
			}
		} else {
			t.Logger.Info("Mode: full backup")
			t.Logger.Debugf("copy(): run QMP transaction: block-dirty-bitmap-add + drive-backup (src=%s, dst=%s", t.srcDisk.BaseName(), t.dstDisk.Path)

			commands := []qmp.Command{
				{"block-dirty-bitmap-add", bitmapArgs},
				{"drive-backup", &backupArgs},
			}
			if err := t.Mon.RunTransaction(t.req.Name, commands, nil); err != nil {
				return err
			}
		}
	} else {
		t.Logger.Info("Mode: full disk copying")

		if err := t.Mon.Run(t.req.Name, qmp.Command{"drive-backup", &backupArgs}, nil); err != nil {
			return err
		}
	}

	// We start collecting statistics after the JOB_STATUS_CHANGE event
	// "JOB_STATUS_CHANGE", "data": {"status": "running", "id": "copy_st_93cb9996"}

	// and continue until the same event with the status == concluded
	// "JOB_STATUS_CHANGE", "data": {"status": "concluded", "id": "copy_st_93cb9996"}}
	// "Concluded" means that a task is done. Not necessarily successful.

	// Then if no errors detected we start waiting the BLOCK_JOB_COMPLETED event.

	errJobNotFound := errors.New("job not found")

	jobID := "copy_" + t.srcDisk.BaseName()

	getJob := func() (*qemu_types.BlockJobInfo, error) {
		jobs := make([]*qemu_types.BlockJobInfo, 0, 1)
		if err := t.Mon.Run(t.req.Name, qmp.Command{"query-block-jobs", nil}, &jobs); err != nil {
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

	if _, err := t.Mon.WaitJobStatusChangeEvent(t.req.Name, timeoutCtx, jobID, "running", uint64(ts.Unix())); err != nil {
		return err
	}

	ts = time.Now()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.Ctx().Done():
			return t.Ctx().Err()
		case <-ticker.C:
		}

		var completed bool

		job, err := getJob()
		switch {
		case err == nil:
		case err == errJobNotFound:
			if _, found, err := t.Mon.FindBlockJobErrorEvent(t.req.Name, jobID, uint64(ts.Unix())); err == nil {
				if found {
					return fmt.Errorf("errors detected during disk mirroring")
				}
			} else {
				return fmt.Errorf("FindBlockJobErrorEvent failed: %s", err)
			}
			if _, found, err := t.Mon.FindBlockJobCompletedEvent(t.req.Name, jobID, uint64(ts.Unix())); err == nil {
				if !found {
					return fmt.Errorf("no completed event found")
				}
			} else {
				return fmt.Errorf("FindBlockJobCompletedEvent failed: %s", err)
			}
			completed = true
		default:
			return err
		}

		if completed {
			t.updateStat(&diskBackupTaskStatUpdate{
				Total:       t.srcSize,
				Remaining:   0,
				Transferred: t.srcSize,
			})

			break
		}

		t.updateStat(&diskBackupTaskStatUpdate{
			Total:       t.srcSize,
			Remaining:   job.Len - job.Offset,
			Transferred: t.srcSize - (job.Len - job.Offset),
		})
	}

	t.Logger.Debug("copy(): completed")

	return nil
}

func (t *DiskBackupTask) mapDeviceToChroot(diskPath string) error {
	chrootDir := filepath.Join(kvmrun.CHROOTDIR, t.req.Name)

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

	return os.Chown(filepath.Join(chrootDir, diskPath), t.vm.C.Uid(), 0)
}
