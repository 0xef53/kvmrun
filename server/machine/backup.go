package machine

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	qemu_types "github.com/0xef53/kvmrun/internal/qemu/types"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	qmp "github.com/0xef53/go-qmp/v2"
)

type DiskBackupOptions struct {
	DiskName    string `json:"disk_name"`
	Target      string `json:"target"`
	Incremental bool   `json:"incremental"`
	ClearBitmap bool   `json:"clear_bitmap"`
}

func (o *DiskBackupOptions) Validate(_ bool) error {
	o.DiskName = strings.TrimSpace(o.DiskName)

	if len(o.DiskName) == 0 {
		return fmt.Errorf("empty diskname")
	}

	o.Target = strings.TrimSpace(o.Target)

	if len(o.Target) == 0 {
		return fmt.Errorf("empty target string")
	}

	return nil
}

func (s *Server) StartDiskBackupProcess(ctx context.Context, vmname string, opts *DiskBackupOptions) (string, error) {
	if opts == nil {
		return "", fmt.Errorf("empty disk backup opts")
	}

	if d, err := kvmrun.NewDisk(opts.DiskName); err == nil {
		opts.DiskName = d.Backend.BaseName()
	}

	t := NewDiskBackupTask(vmname, opts)

	t.Server = s

	taskOpts := []task.TaskOption{
		server.WithUniqueLabel(vmname + "/disk-backup/" + opts.DiskName),
		server.WithGroupLabel(vmname),
		server.WithGroupLabel(vmname + "/long-running"),
	}

	tid, err := s.TaskStart(ctx, t, nil, taskOpts...)
	if err != nil {
		return "", fmt.Errorf("cannot start disk backup: %w", err)
	}

	return tid, nil
}

type DiskBackupStatDetails struct {
	Disk *DataTransferStat
}

type DiskBackupTask struct {
	*task.GenericTask
	*Server

	targets map[string]task.OperationMode

	// Arguments
	vmname string
	opts   *DiskBackupOptions

	// Do not set manually next fields !
	vm *kvmrun.Machine

	srcSize uint64
	srcDisk *kvmrun.Disk
	dstDisk *kvmrun.Disk

	details  *DiskBackupStatDetails
	statfile string

	mu sync.Mutex
}

func NewDiskBackupTask(vmname string, opts *DiskBackupOptions) *DiskBackupTask {
	return &DiskBackupTask{
		GenericTask: new(task.GenericTask),

		targets: server.BlockBackupOperations(vmname, opts.DiskName),
		vmname:  vmname,
		opts:    opts,
	}
}

func (t *DiskBackupTask) Targets() map[string]task.OperationMode { return t.targets }

func (t *DiskBackupTask) BeforeStart(_ interface{}) error {
	if t.opts == nil {
		return fmt.Errorf("empty disk backup opts")
	} else {
		if err := t.opts.Validate(true); err != nil {
			return err
		}
	}

	if vm, err := t.Server.MachineGet(t.vmname, true); err == nil {
		t.vm = vm
	} else {
		return err
	}

	vmstate, err := t.MachineGetStatus(t.vm)
	if err != nil {
		return err
	}

	switch vmstate {
	case kvmrun.StatePaused, kvmrun.StateRunning:
		if t.vm.R == nil {
			return fmt.Errorf("unexpected error: QEMU instance not found")
		}
	default:
		return fmt.Errorf("incompatible machine state: %d", vmstate)
	}

	var srcDisk *kvmrun.Disk

	if d := t.vm.R.DiskGet(t.opts.DiskName); d != nil {
		srcDisk = d
	} else {
		return &kvmrun.NotConnectedError{Source: "instance_qemu", Object: t.opts.DiskName}
	}

	if d := t.vm.R.DiskGet(t.opts.Target); d != nil {
		return fmt.Errorf("target device is connected to the current machine: %s", t.opts.Target)
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

	dstDisk, err := kvmrun.NewDisk(t.opts.Target)
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
			return fmt.Errorf("target size is smaller than source (%d < %d)", dstSize, srcSize)
		}
	}

	t.srcSize = srcSize
	t.srcDisk = srcDisk
	t.dstDisk = dstDisk

	// Init stat fields
	t.details = &DiskBackupStatDetails{
		Disk: new(DataTransferStat),
	}

	hashname := fmt.Sprintf("%x", md5.Sum([]byte(t.vmname+"/disk-backup/"+srcDisk.Backend.BaseName())))

	t.statfile = filepath.Join(kvmrun.CHROOTDIR, t.vmname, ".tasks", hashname)

	if err := os.Remove(t.statfile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (t *DiskBackupTask) Stat() *task.TaskStat {
	t.mu.Lock()
	defer t.mu.Unlock()

	st := t.GenericTask.Stat()

	st.Details = t.details

	return st
}

func (t *DiskBackupTask) updateStat(total, remain, sent uint64, speed int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if total == 0 {
		return
	}

	t.details.Disk = &DataTransferStat{
		Total:       total,
		Remaining:   remain,
		Transferred: sent,
		Progress:    int(sent * 100 / total),
		Speed:       speed,
	}

	t.SetProgress(int(t.details.Disk.Progress))
}

func (t *DiskBackupTask) writeStatFile(taskErr error) error {
	st := t.Stat()

	if taskErr == nil {
		st.State = task.StateCompleted
	} else {
		st.State = task.StateFailed
		st.StateDesc = taskErr.Error()
	}

	statFileData := server.TaskStatFile{
		Label: t.vmname + "/disk-backup/" + t.srcDisk.Backend.BaseName(),
		Stat:  st,
	}

	b, err := json.MarshalIndent(statFileData, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(t.statfile, b, 0644)
}

func (t *DiskBackupTask) Main() (err error) {
	defer func() {
		if _err := t.writeStatFile(err); err != nil {
			t.Logger.Errorf("Failed to save task state to a special file: %s", _err)
		}
	}()

	return t.copy()
}

func (t *DiskBackupTask) OnFailure(taskErr error) {
	opts := struct {
		Device string `json:"device"`
		Force  bool   `json:"force,omitempty"`
	}{
		Device: "copy_" + t.srcDisk.BaseName(),
		Force:  true,
	}

	if err := t.Mon.Run(t.vmname, qmp.Command{Name: "block-job-cancel", Arguments: &opts}, nil); err != nil {
		// non-fatal error. Just printing
		t.Logger.Errorf("OnFailureHook: forced block-job-cancel failed: %s", err)
	}
}

func (t *DiskBackupTask) copy() error {
	t.Logger.Debug("copy(): main process started")

	if t.dstDisk.IsLocal() {
		defer os.Remove(filepath.Join(kvmrun.CHROOTDIR, t.vmname, t.dstDisk.Path))

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

	if t.opts.Incremental {
		bitmapArgs := qemu_types.BlockDirtyBitmapOptions{
			Node: t.srcDisk.BaseName(),
			Name: "backup",
		}

		if t.srcDisk.HasBitmap {
			if t.opts.ClearBitmap {
				t.Logger.Info("Mode: full backup (with bitmap reset)")
				t.Logger.Debugf("copy(): run QMP transaction: block-dirty-bitmap-clear + drive-backup (src=%s, dst=%s", t.srcDisk.BaseName(), t.dstDisk.Path)

				commands := []qmp.Command{
					{Name: "block-dirty-bitmap-clear", Arguments: &bitmapArgs},
					{Name: "drive-backup", Arguments: &backupArgs},
				}

				if err := t.Server.Mon.RunTransaction(t.vmname, commands, nil); err != nil {
					return err
				}
			} else {
				t.Logger.Info("Mode: incremental backup")
				t.Logger.Debugf("copy(): run QMP command: drive-backup (src=%s, dst=%s)", t.srcDisk.BaseName(), t.dstDisk.Path)

				backupArgs.Sync = "incremental"
				backupArgs.Bitmap = "backup"

				if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "drive-backup", Arguments: &backupArgs}, nil); err != nil {
					return err
				}
			}
		} else {
			t.Logger.Info("Mode: full backup")
			t.Logger.Debugf("copy(): run QMP transaction: block-dirty-bitmap-add + drive-backup (src=%s, dst=%s", t.srcDisk.BaseName(), t.dstDisk.Path)

			commands := []qmp.Command{
				{Name: "block-dirty-bitmap-add", Arguments: bitmapArgs},
				{Name: "drive-backup", Arguments: &backupArgs},
			}

			if err := t.Server.Mon.RunTransaction(t.vmname, commands, nil); err != nil {
				return err
			}
		}
	} else {
		t.Logger.Info("Mode: full disk copying")

		if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "drive-backup", Arguments: &backupArgs}, nil); err != nil {
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

		if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "query-block-jobs", Arguments: nil}, &jobs); err != nil {
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

	if _, err := t.Server.Mon.WaitJobStatusChangeEvent(t.vmname, timeoutCtx, jobID, "running", uint64(ts.Unix())); err != nil {
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

		switch err {
		case nil:
		case errJobNotFound:
			if _, found, err := t.Server.Mon.FindBlockJobErrorEvent(t.vmname, jobID, uint64(ts.Unix())); err == nil {
				if found {
					return fmt.Errorf("errors detected during disk mirroring")
				}
			} else {
				return fmt.Errorf("FindBlockJobErrorEvent failed: %s", err)
			}

			if _, found, err := t.Server.Mon.FindBlockJobCompletedEvent(t.vmname, jobID, uint64(ts.Unix())); err == nil {
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
			// (total, remain, sent and speed)
			t.updateStat(t.srcSize, 0, t.srcSize, 0)

			break
		}

		// (total, remain, sent and speed)
		t.updateStat(t.srcSize, job.Len-job.Offset, t.srcSize-(job.Len-job.Offset), 0)
	}

	t.Logger.Debug("copy(): completed")

	return nil
}

func (t *DiskBackupTask) mapDeviceToChroot(diskPath string) error {
	chrootDir := filepath.Join(kvmrun.CHROOTDIR, t.vmname)

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

	return os.Chown(filepath.Join(chrootDir, diskPath), t.vm.C.UID(), 0)
}
