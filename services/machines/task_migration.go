package machines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/0xef53/kvmrun/internal/grpcclient"
	"github.com/0xef53/kvmrun/internal/osuser"
	qemu_types "github.com/0xef53/kvmrun/internal/qemu/types"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/internal/types"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/services"

	m_pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	s_pb "github.com/0xef53/kvmrun/api/services/system/v1"
	t_pb "github.com/0xef53/kvmrun/api/services/tasks/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"

	qmp "github.com/0xef53/go-qmp/v2"
	"golang.org/x/sync/errgroup"
)

type MachineMigrationTask struct {
	*task.GenericTask
	*services.ServiceServer

	mu sync.Mutex

	vm  *kvmrun.Machine
	req *m_pb.StartMigrationRequest

	movingDisks   kvmrun.DiskPool
	dstServerAddr net.IP
	incomingReq   *s_pb.StartIncomingMachineRequest
	requisites    *pb_types.IncomingMachineRequisites

	details *types.MachineMigrationDetails
}

func NewMachineMigrationTask(req *m_pb.StartMigrationRequest, ss *services.ServiceServer, vm *kvmrun.Machine) *MachineMigrationTask {
	return &MachineMigrationTask{
		GenericTask:   new(task.GenericTask),
		ServiceServer: ss,
		req:           req,
		vm:            vm,
	}
}

func (t *MachineMigrationTask) GetNS() string { return "machine-migration" }

func (t *MachineMigrationTask) GetKey() string { return t.req.Name + "::" }

func (t *MachineMigrationTask) Stat() *task.TaskStat {
	t.mu.Lock()
	defer t.mu.Unlock()

	st := t.GenericTask.Stat()

	st.Details = t.details

	return st
}

func (t *MachineMigrationTask) BeforeStart(_ interface{}) error {
	if t.vm.R == nil {
		return fmt.Errorf("not running: %s", t.req.Name)
	}

	if ips, err := net.LookupIP(t.req.DstServer); err == nil {
		t.dstServerAddr = ips[0]
	} else {
		return err
	}

	attachedConfDisks := t.vm.C.(*kvmrun.InstanceConf).Disks
	//attachedQemuDisks := t.vm.R.(*kvmrun.InstanceQemu).Disks
	attachedQemuDisks := t.vm.R.GetDisks()

	// t.req.Disks may contain short names and that's OK
	if len(t.req.Disks) > 0 {
		t.movingDisks = make(kvmrun.DiskPool, 0, len(t.req.Disks))
		for _, diskPath := range t.req.Disks {
			if d := attachedQemuDisks.Get(diskPath); d != nil {
				t.movingDisks = append(t.movingDisks, *d)
			} else {
				return &kvmrun.NotConnectedError{"instance_qemu", diskPath}
			}
		}
	}

	if len(t.req.Overrides.Disks) > 0 {
		swp := make(map[string]string)

		// The keys of t.req.Overrides.Disks may be presented as short names.
		// But the values must be specified as fully qualified names
		for orig, ovrd := range t.req.Overrides.Disks {
			if d := attachedQemuDisks.Get(orig); d != nil {
				// "orig" could be a short name (BaseName) of disk,
				// but we need the full name. So we use d.Path for that.
				swp[d.Path] = ovrd
			} else {
				return &kvmrun.NotConnectedError{"instance_qemu", orig}
			}

			if d := attachedConfDisks.Get(orig); d == nil {
				// It's OK now if "orig" is not in the configuration,
				// but in the future we will also need to migrate offline disks
			}
		}

		t.req.Overrides.Disks = swp
	}

	increq, err := t.newIncomingRequest()
	if err != nil {
		return fmt.Errorf("failed to build incoming request: %s", err)
	}

	conn, err := grpcclient.NewConn(t.dstServerAddr.String(), t.AppConf.Common.TLSConfig, true)
	if err != nil {
		return err
	}
	defer conn.Close()

	resp, err := s_pb.NewSystemServiceClient(conn).StartIncomingMachine(t.Ctx(), increq)
	if err != nil {
		return err
	}

	t.incomingReq = increq
	t.requisites = resp.Requisites

	t.details = &types.MachineMigrationDetails{
		DstServer: t.req.DstServer,
		VMState:   new(types.DataTransferStat),
		Disks:     make(map[string]*types.DataTransferStat),
	}
	for _, d := range t.movingDisks {
		t.details.Disks[d.Path] = &types.DataTransferStat{Total: d.QemuVirtualSize}
	}

	return nil
}

func (t *MachineMigrationTask) Main() error {
	// The migration of virt.machine state and its disks
	// are running in the parallel goroutines.
	// These variables are to syncronize the states between goroutines.
	allDisksReady := make(chan struct{})
	vmstateMigrated := make(chan struct{})

	group, ctx := errgroup.WithContext(t.Ctx())

	// Start mirroring the firmware flash device (if needed)
	if fwflash := t.vm.C.GetFirmwareFlash(); fwflash != nil {
		firmwareFlashReady := make(chan struct{})

		group.Go(func() error {
			return StartStorageProcessor(ctx, t, []kvmrun.Disk{*fwflash}, firmwareFlashReady, vmstateMigrated)
		})

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-firmwareFlashReady:
			break
		}
	}

	// Start mirroring the specified disks
	if len(t.req.Disks) > 0 {
		group.Go(func() error { return t.mirrorDisks(ctx, allDisksReady, vmstateMigrated) })
	} else {
		close(allDisksReady)
	}

	// Start the migration of virt.machine state
	group.Go(func() error { return t.migrateVMState(ctx, allDisksReady, vmstateMigrated) })

	// ... and wait for their completion
	return group.Wait()
}

func (t *MachineMigrationTask) OnSuccess() error {
	os.WriteFile(t.MachineDownFile(t.req.Name), []byte(""), 0644)

	// Forced shutdown when migration is successful
	if err := t.Mon.Run(t.req.Name, qmp.Command{"quit", nil}, nil); err != nil {
		t.Logger.Error(err.Error())
	}

	testRemoteMachine := func() error {
		conn, err := grpcclient.NewConn(t.dstServerAddr.String(), t.AppConf.Common.TLSConfig, true)
		if err != nil {
			return err
		}
		defer conn.Close()

		req := m_pb.GetMachineRequest{
			Name: t.req.Name,
		}

		resp, err := m_pb.NewMachineServiceClient(conn).Get(t.Ctx(), &req)
		if err != nil {
			return err
		}

		if t.requisites.Pid != resp.Machine.Pid {
			return fmt.Errorf("remote machine PID has changed since it was started: %d != %d", t.requisites.Pid, resp.Machine.Pid)
		}

		if t.incomingReq.TurnOffAfter {
			switch resp.Machine.State {
			case pb_types.MachineState_INACTIVE, pb_types.MachineState_SHUTDOWN:
			default:
				return fmt.Errorf("remote machine must be inactive (TurnOffAfter == true) but current state is %s", resp.Machine.State)
			}
		} else {
			switch resp.Machine.State {
			case pb_types.MachineState_RUNNING:
			default:
				return fmt.Errorf("remote machine is not running: current state is %s", resp.Machine.State)
			}
		}

		return nil
	}

	if t.req.RemoveAfter {
		t.Logger.Info("The machine will be removed as requested (RemoveAfter == true)")

		err := func() error {
			if err := testRemoteMachine(); err != nil {
				return err
			}

			if err := t.SystemCtl.StopAndWait(t.MachineToUnit(t.req.Name), 30*time.Second, nil); err != nil {
				return fmt.Errorf("failed to shutdown %s: %s", t.req.Name, err)
			}

			if err := t.SystemCtl.Disable(t.MachineToUnit(t.req.Name)); err != nil {
				return err
			}

			osuser.RemoveUser(t.req.Name)

			dirs := []string{
				filepath.Join(kvmrun.CONFDIR, t.req.Name),
				filepath.Join(kvmrun.LOGDIR, t.req.Name),
			}
			for _, d := range dirs {
				if err := os.RemoveAll(d); err != nil {
					return err
				}
			}

			return nil
		}()
		if err != nil {
			t.Logger.Errorf("Failed to remove configuretion of %s: %s", t.req.Name, err)
		}
	}

	return nil
}

func (t *MachineMigrationTask) OnFailure() error {
	for _, d := range t.movingDisks {
		opts := struct {
			Device string `json:"device"`
			Force  bool   `json:"force,omitempty"`
		}{
			Device: "migr_" + d.BaseName(),
			Force:  true,
		}
		if err := t.Mon.Run(t.req.Name, qmp.Command{"block-job-cancel", &opts}, nil); err != nil {
			// Non-fatal error. Just printing
			t.Logger.Errorf("OnFailureHook: forced block-job-cancel failed: %s", err)
		}
	}

	if err := t.Mon.Run(t.req.Name, qmp.Command{"migrate_cancel", nil}, nil); err != nil {
		// Non-fatal error. Just printing
		t.Logger.Errorf("OnFailureHook: forced migrate_cancel failed: %s", err)
	}

	if conn, err := grpcclient.NewConn(t.dstServerAddr.String(), t.AppConf.Common.TLSConfig, true); err == nil {
		defer conn.Close()

		req := t_pb.CancelTaskRequest{
			Key: "machine-incoming:" + t.req.Name + "::",
		}
		if _, err := t_pb.NewTaskServiceClient(conn).Cancel(t.Ctx(), &req); err == nil {
			t.Logger.Info("OnFailureHook: the remote incoming task was successfully cancelled")
		} else {
			// Non-fatal error. Just printing
			t.Logger.Errorf("OnFailureHook: failed to cancel the remote task (%s): %s", req.Key, err)
		}
	} else {
		// Non-fatal error. Just printing
		t.Logger.Errorf("OnFailureHook: failed to cancel the remote task: %s", err)
	}

	return nil
}

type machineMigrationTaskStatUpdate struct {
	Object      string
	Name        string
	Total       uint64
	Remaining   uint64
	Transferred uint64
	Speed       int32
}

func (t *MachineMigrationTask) updateStat(u *machineMigrationTaskStatUpdate) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch u.Object {
	case "vmstate":
		if u.Total > 0 {
			t.details.VMState = &types.DataTransferStat{
				Total:       u.Total,
				Remaining:   u.Remaining,
				Transferred: u.Transferred,
				Progress:    int32(u.Transferred * 100 / u.Total),
				Speed:       u.Speed,
			}
		}
	case "disk":
		if u.Total > 0 {
			t.details.Disks[u.Name] = &types.DataTransferStat{
				Total:       u.Total,
				Remaining:   u.Remaining,
				Transferred: u.Transferred,
				Progress:    int32(u.Transferred * 100 / u.Total),
				Speed:       int32(((u.Transferred - t.details.Disks[u.Name].Transferred) * 8) / 1 >> (10 * 2)), // mbit/s
			}
		}
	}

	total := t.details.VMState.Progress

	for _, d := range t.details.Disks {
		total += d.Progress
	}

	t.SetProgress(total / int32(1+len(t.details.Disks)))
}

func (t *MachineMigrationTask) migrateVMState(ctx context.Context, allDisksReady, vmstateMigrated chan struct{}) error {
	defer close(vmstateMigrated)

	t.Logger.Debug("migrateVMState(): start QEMU memory+state migration")
	t.Logger.Debug("migrateVMState(): wait for disks synchronization ...")

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-allDisksReady:
		break
	}

	t.Logger.Debug("migrateVMState(): disks are synchronized")
	t.Logger.Debug("migrateVMState(): run QMP command: migrate-set-capabilities")

	// Capabilities && Parameters
	capsArgs := struct {
		Capabilities []qemu_types.MigrationCapabilityStatus `json:"capabilities"`
	}{
		Capabilities: []qemu_types.MigrationCapabilityStatus{
			{"xbzrle", true},
			{"auto-converge", true},
			{"compress", false},
			{"block", false},
			{"dirty-bitmaps", true},
		},
	}
	if err := t.Mon.Run(t.req.Name, qmp.Command{"migrate-set-capabilities", &capsArgs}, nil); err != nil {
		return err
	}

	t.Logger.Debug("migrateVMState(): run QMP command: migrate-set-parameters")

	paramsArgs := qemu_types.MigrateSetParameters{
		MaxBandwidth:    8589934592,
		XbzrleCacheSize: 536870912,
	}
	if err := t.Mon.Run(t.req.Name, qmp.Command{"migrate-set-parameters", &paramsArgs}, nil); err != nil {
		return err
	}

	t.Logger.Debug("migrateVMState(): run QMP command: migrate; waiting for QEMU migration ...")

	// Run
	args := struct {
		URI string `json:"uri"`
	}{
		URI: fmt.Sprintf("tcp:%s:%d", t.dstServerAddr, t.requisites.IncomingPort),
	}

	if err := t.Mon.Run(t.req.Name, qmp.Command{"migrate", &args}, nil); err != nil {
		return err
	}

LOOP:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		mi := &qemu_types.MigrationInfo{}

		if err := t.Mon.Run(t.req.Name, qmp.Command{"query-migrate", nil}, mi); err != nil {
			return err
		}

		switch mi.Status {
		case "active", "postcopy-active", "completed":
			t.updateStat(&machineMigrationTaskStatUpdate{
				Object:      "vmstate",
				Total:       mi.Ram.Total,
				Remaining:   mi.Ram.Remaining,
				Transferred: mi.Ram.Total - mi.Ram.Remaining,
				Speed:       int32(mi.Ram.Speed),
			})
		}

		switch mi.Status {
		case "active", "postcopy-active":
		case "completed":
			break LOOP
		case "failed":
			return fmt.Errorf("QEMU migration failed: %s", mi.ErrDesc)
		case "cancelled":
			return fmt.Errorf("QEMU migration cancelled by QMP command")
		}

		time.Sleep(time.Second * 1)
	}

	t.Logger.Debug("migrateVMState(): completed")

	return nil
}

func (t *MachineMigrationTask) mirrorDisks(ctx context.Context, allDisksReady, vmstateMigrated chan struct{}) error {
	t.Logger.Debug("mirrorDisks(): start disks mirroring process")

	errJobNotFound := errors.New("job not found")

	getJob := func(jobID string) (*qemu_types.BlockJobInfo, error) {
		jobs := make([]*qemu_types.BlockJobInfo, 0, len(t.req.Disks))
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

	waitForReady := func(ctx context.Context, ts time.Time, d *kvmrun.Disk) error {
		jobID := "migr_" + d.BaseName()

		// Stat will be available after the job status changes to running
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := t.Mon.WaitJobStatusChangeEvent(t.req.Name, timeoutCtx, jobID, "running", uint64(ts.Unix())); err != nil {
			return err
		}

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}

			job, err := getJob(jobID)
			if err != nil {
				// No errors should be here, even errJobNotfound
				return err
			}

			t.updateStat(&machineMigrationTaskStatUpdate{
				Object:      "disk",
				Name:        d.Path,
				Total:       d.QemuVirtualSize,
				Remaining:   job.Len - job.Offset,
				Transferred: d.QemuVirtualSize - (job.Len - job.Offset),
			})

			if job.Ready {
				break
			}
		}
		return nil
	}

	group1, ctx1 := errgroup.WithContext(ctx)

	// Start the mirroring all disks.
	// For each disk, we run a separate goroutine,
	// where the Ready status is expected.
	for _, d := range t.movingDisks {
		t.Logger.Debugf("mirrorDisks(): run QMP command: drive-mirror; name=%s, remote_addr=%s:%d", d.BaseName(), t.dstServerAddr.String(), t.requisites.NBDPort)

		d := d // shadow to be captured by closure
		dstName := d.BaseName()

		if ovrd, ok := t.req.Overrides.Disks[d.Path]; ok {
			if _d, err := kvmrun.NewDisk(ovrd); err == nil {
				dstName = _d.BaseName()
			} else {
				return err
			}
		}

		ts := time.Now()

		args := t.newQemuDriveMirrorOpts(t.dstServerAddr.String(), int(t.requisites.NBDPort), d.BaseName(), dstName)
		if err := t.Mon.Run(t.req.Name, qmp.Command{"drive-mirror", &args}, nil); err != nil {
			return err
		}

		group1.Go(func() error { return waitForReady(ctx1, ts, &d) })
	}

	t.Logger.Debug("mirrorDisks(): wait for disks synchronization ...")

	if err := group1.Wait(); err != nil {
		return err
	}

	t.Logger.Debug("mirrorDisks(): disks are synchronized")

	// All disks are ready. Notify migrateVMState() goroutine about this
	close(allDisksReady)

	t.Logger.Debug("mirrorDisks(): channel allDiskReady is closed now")

	// Stat update and wait for migrateVMState() is completed
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

LOOP:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-vmstateMigrated:
			ticker.Stop()
			break LOOP
		case <-ticker.C:
		}

		for _, d := range t.movingDisks {
			job, err := getJob("migr_" + d.BaseName())
			if err != nil {
				return err
			}

			t.updateStat(&machineMigrationTaskStatUpdate{
				Object:      "disk",
				Name:        d.Path,
				Total:       d.QemuVirtualSize,
				Remaining:   job.Len - job.Offset,
				Transferred: d.QemuVirtualSize - (job.Len - job.Offset),
			})
		}
	}

	t.Logger.Debug("mirrorDisks(): QEMU migration is completed. Wait for total disks synchronization ...")

	waitForCompleted := func(ctx context.Context, ts time.Time, d *kvmrun.Disk) error {
		jobID := "migr_" + d.BaseName()

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}

			job, err := getJob(jobID)
			if err != nil && err != errJobNotFound {
				return fmt.Errorf("failed to complete disk mirroring for %s: %s", d.BaseName(), err)
			}
			// Ok, job completed
			if job == nil {
				if _, found, err := t.Mon.FindBlockJobErrorEvent(t.req.Name, jobID, uint64(ts.Unix())); err == nil {
					if found {
						return fmt.Errorf("errors detected during disk mirroring: %s", d.BaseName())
					}
				} else {
					return fmt.Errorf("FindBlockJobErrorEvent failed: %s: %s", d.BaseName(), err)
				}
				if _, found, err := t.Mon.FindBlockJobCompletedEvent(t.req.Name, jobID, uint64(ts.Unix())); err == nil {
					if !found {
						return fmt.Errorf("no completed event found: %s", d.BaseName())
					}
				} else {
					return fmt.Errorf("FindBlockJobCompletedEvent failed: %s: %s", d.BaseName(), err)
				}
				break
			}
		}
		return nil
	}

	t.Logger.Debug("mirrorDisks(): finish all drive-mirror processes ...")

	group2, ctx2 := errgroup.WithContext(ctx)

	// Stop the mirroring
	for _, d := range t.movingDisks {
		t.Logger.Debugf("mirrorDisks(): run QMP command: block-job-complete; name=%s", d.BaseName())
		d := d // shadow for closure
		jobID := struct {
			Device string `json:"device"`
		}{
			Device: "migr_" + d.BaseName(),
		}
		ts := time.Now()
		if err := t.Mon.Run(t.req.Name, qmp.Command{"block-job-complete", &jobID}, nil); err != nil {
			// Non-fatal error
			t.Logger.Error(err.Error())
			continue
		}
		group2.Go(func() error { return waitForCompleted(ctx2, ts, &d) })
	}

	switch err := group2.Wait(); {
	case err == nil:
	case err == context.Canceled, err == context.DeadlineExceeded:
		return err
	default:
		// The migration is actually done,
		// so we don't care about these errors
		t.Logger.Error(err.Error())
	}

	t.Logger.Debug("mirrorDisks(): completed")

	return nil
}

func (t *MachineMigrationTask) newQemuDriveMirrorOpts(addr string, port int, srcName, dstName string) *qemu_types.DriveMirrorOptions {
	opts := qemu_types.DriveMirrorOptions{
		JobID:    fmt.Sprintf("migr_%s", srcName),
		Device:   srcName,
		Target:   fmt.Sprintf("nbd:%s:%d:exportname=%s", addr, port, dstName),
		Format:   "nbd",
		Sync:     "full",
		Mode:     "existing",
		CopyMode: "background",
	}

	if t.vm.R.GetQemuVersion().Int() >= 60000 { // >= 6.x.x
		opts.CopyMode = "write-blocking"
	}

	return &opts
}

func (t *MachineMigrationTask) newIncomingRequest() (*s_pb.StartIncomingMachineRequest, error) {
	diskSizes := make(map[string]uint64)

	// Ключи в Overrides.Disks уже проверены и приведены к полному имени
	for _, d := range t.movingDisks {
		if ovrd, ok := t.req.Overrides.Disks[d.Path]; ok {
			diskSizes[ovrd] = d.QemuVirtualSize
		} else {
			diskSizes[d.Path] = d.QemuVirtualSize
		}
	}

	manifest, err := func() ([]byte, error) {
		if len(t.req.Overrides.Disks) > 0 {
			tmp := struct {
				Name string               `json:"name"`
				C    *kvmrun.InstanceConf `json:"conf"`
				R    *kvmrun.InstanceQemu `json:"run"`
			}{}

			b, err := json.Marshal(t.vm)
			if err != nil {
				return nil, err
			}

			if err := json.Unmarshal(b, &tmp); err != nil {
				return nil, err
			}

			for idx := range tmp.C.Disks {
				if ovrd, ok := t.req.Overrides.Disks[tmp.C.Disks[idx].Path]; ok {
					tmp.C.Disks[idx].Path = ovrd
				}
			}

			for idx := range tmp.R.Disks {
				if ovrd, ok := t.req.Overrides.Disks[tmp.R.Disks[idx].Path]; ok {
					tmp.R.Disks[idx].Path = ovrd
				}
			}

			return json.Marshal(tmp)
		}

		return json.Marshal(t.vm)
	}()
	if err != nil {
		return nil, err
	}

	var st qemu_types.StatusInfo

	if err := t.Mon.Run(t.req.Name, qmp.Command{"query-status", nil}, &st); err != nil {
		return nil, err
	}

	// Some extra files that may contain additional
	// configuration such as network settings.
	extraFiles, err := t.extraFiles()
	if err != nil {
		return nil, err
	}

	return &s_pb.StartIncomingMachineRequest{
		Name:         t.req.Name,
		Manifest:     manifest,
		ListenAddr:   t.dstServerAddr.String(),
		Disks:        diskSizes,
		CreateDisks:  t.req.CreateDisks,
		ExtraFiles:   extraFiles,
		TurnOffAfter: st.Status == "paused",
	}, nil
}

// extraFiles returns a map including the contents of some extra files
// placed in the virtual machine directory.
// These files may contain additional configuration such as network settings
// (config_network).
func (t *MachineMigrationTask) extraFiles() (map[string][]byte, error) {
	r := regexp.MustCompile(`^(extra|comment|cloudinit_drive|ci_drive|config_[[:alnum:]]*|[\.\_[:alnum:]]*_config)$`)

	vmdir := filepath.Join(kvmrun.CONFDIR, t.req.Name)

	files, err := os.ReadDir(vmdir)
	if err != nil {
		return nil, err
	}

	extraFiles := make(map[string][]byte)

	for _, f := range files {
		if !f.Type().IsRegular() || f.Name() == "config" || !r.MatchString(f.Name()) {
			continue
		}

		c, err := os.ReadFile(filepath.Join(vmdir, f.Name()))
		if err != nil {
			return nil, err
		}

		extraFiles[f.Name()] = c
	}

	return extraFiles, nil
}

type MachineMigrationTask_StorageProcessor struct {
	t *MachineMigrationTask

	ctx context.Context

	disks                kvmrun.DiskPool
	readyToComplete      chan struct{}
	machineStateMigrated chan struct{}

	errJobNotFound error
}

func StartStorageProcessor(ctx context.Context, t *MachineMigrationTask, disks kvmrun.DiskPool, ready, stateMigrated chan struct{}) error {
	// TODO: add sub ID to make it easier to search in logs

	defer func() {
		select {
		case <-ready:
		default:
			close(ready)
		}
	}()

	processor := MachineMigrationTask_StorageProcessor{
		t:                    t,
		ctx:                  ctx,
		disks:                disks,
		readyToComplete:      ready,
		machineStateMigrated: stateMigrated,
		errJobNotFound:       errors.New("job not found"),
	}

	return processor.run()
}

func (p *MachineMigrationTask_StorageProcessor) run() error {
	diskNames := make([]string, 0, len(p.disks))
	for _, d := range p.disks {
		diskNames = append(diskNames, d.BaseName())
	}

	p.t.Logger.Infof("Start disks mirroring process (%s)", strings.Join(diskNames, ", "))

	group1, ctx1 := errgroup.WithContext(p.ctx)

	// Start the mirroring all disks.
	// For each disk, we run a separate goroutine,
	// where the Ready status is expected.
	for _, d := range p.disks {
		p.t.Logger.Infof("Run QMP command: drive-mirror; name=%s, remote_addr=%s:%d", d.BaseName(), p.t.dstServerAddr.String(), p.t.requisites.NBDPort)

		d := d // shadow to be captured by closure
		dstName := d.BaseName()

		if ovrd, ok := p.t.req.Overrides.Disks[d.Path]; ok {
			if _d, err := kvmrun.NewDisk(ovrd); err == nil {
				dstName = _d.BaseName()
			} else {
				return err
			}
		}

		ts := time.Now()

		if err := p.t.Mon.Run(p.t.req.Name, qmp.Command{"drive-mirror", p.newMirrorOpts(d.BaseName(), dstName)}, nil); err != nil {
			return fmt.Errorf("failed to start mirroring (%s): %w", d.BaseName(), err)
		}

		group1.Go(func() error { return p.waitForReady(ctx1, ts, &d) })
	}

	p.t.Logger.Info("Wait for disks synchronization ...")

	if err := group1.Wait(); err != nil {
		return err
	}

	// All disks are ready. Notify dependent processes about this
	close(p.readyToComplete)

	p.t.Logger.Info("All disks are synchronized")

	// Stat update and wait for machine state is migrated
	if err := p.waitMachineStateMigrated(); err != nil {
		return err
	}

	p.t.Logger.Debug("QEMU migration is completed. Wait for total disks synchronization ...")

	group2, ctx2 := errgroup.WithContext(p.ctx)

	// Stop the mirroring
	for _, d := range p.disks {
		p.t.Logger.Debugf("Run QMP command: block-job-complete; name=%s", d.BaseName())

		d := d // shadow for closure

		jobID := struct {
			Device string `json:"device"`
		}{
			Device: "migr_" + d.BaseName(),
		}

		ts := time.Now()

		if err := p.t.Mon.Run(p.t.req.Name, qmp.Command{"block-job-complete", &jobID}, nil); err != nil {
			// Non-fatal error, just printing
			p.t.Logger.Errorf("Failed to stop mirroring (%s): %s", d.BaseName(), err.Error())
			continue
		}

		group2.Go(func() error { return p.waitForCompleted(ctx2, ts, &d) })
	}

	switch err := group2.Wait(); {
	case err == nil:
	case err == context.Canceled, err == context.DeadlineExceeded:
		return err
	default:
		// The migration is actually done,
		// so we don't care about these errors
		p.t.Logger.Error(err.Error())
	}

	p.t.Logger.Infof("Disks mirroring process completed (%s)", strings.Join(diskNames, ", "))

	return nil
}

func (p *MachineMigrationTask_StorageProcessor) getJob(jobID string) (*qemu_types.BlockJobInfo, error) {
	jobs := make([]*qemu_types.BlockJobInfo, 0, len(p.disks))

	if err := p.t.Mon.Run(p.t.req.Name, qmp.Command{"query-block-jobs", nil}, &jobs); err != nil {
		return nil, err
	}

	for _, j := range jobs {
		if j.Device == jobID {
			return j, nil
		}
	}
	return nil, p.errJobNotFound
}

func (p *MachineMigrationTask_StorageProcessor) waitForReady(ctx context.Context, ts time.Time, d *kvmrun.Disk) error {
	jobID := "migr_" + d.BaseName()

	// Stat will be available after the job status changes to running
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := p.t.Mon.WaitJobStatusChangeEvent(p.t.req.Name, timeoutCtx, jobID, "running", uint64(ts.Unix())); err != nil {
		return err
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		job, err := p.getJob(jobID)
		if err != nil {
			// No errors should be here, even errJobNotfound
			return err
		}

		if d.BaseName() != "fwflash" {
			p.t.updateStat(&machineMigrationTaskStatUpdate{
				Object:      "disk",
				Name:        d.Path,
				Total:       d.QemuVirtualSize,
				Remaining:   job.Len - job.Offset,
				Transferred: d.QemuVirtualSize - (job.Len - job.Offset),
			})
		}

		if job.Ready {
			break
		}
	}

	return nil
}

func (p *MachineMigrationTask_StorageProcessor) waitForCompleted(ctx context.Context, ts time.Time, d *kvmrun.Disk) error {
	jobID := "migr_" + d.BaseName()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		job, err := p.getJob(jobID)
		if err != nil && err != p.errJobNotFound {
			return fmt.Errorf("failed to complete disk mirroring for %s: %s", d.BaseName(), err)
		}

		// Ok, job completed
		if job == nil {
			if _, found, err := p.t.Mon.FindBlockJobErrorEvent(p.t.req.Name, jobID, uint64(ts.Unix())); err == nil {
				if found {
					return fmt.Errorf("errors detected during disk mirroring: %s", d.BaseName())
				}
			} else {
				return fmt.Errorf("FindBlockJobErrorEvent failed: %s: %s", d.BaseName(), err)
			}

			if _, found, err := p.t.Mon.FindBlockJobCompletedEvent(p.t.req.Name, jobID, uint64(ts.Unix())); err == nil {
				if !found {
					return fmt.Errorf("no completed event found: %s", d.BaseName())
				}
			} else {
				return fmt.Errorf("FindBlockJobCompletedEvent failed: %s: %s", d.BaseName(), err)
			}

			break
		}
	}

	return nil
}

func (p *MachineMigrationTask_StorageProcessor) waitMachineStateMigrated() error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

LOOP:
	for {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		case <-p.machineStateMigrated:
			ticker.Stop()
			break LOOP
		case <-ticker.C:
		}

		for _, d := range p.disks {
			job, err := p.getJob("migr_" + d.BaseName())
			if err != nil {
				return err
			}

			if d.BaseName() != "fwflash" {
				p.t.updateStat(&machineMigrationTaskStatUpdate{
					Object:      "disk",
					Name:        d.Path,
					Total:       d.QemuVirtualSize,
					Remaining:   job.Len - job.Offset,
					Transferred: d.QemuVirtualSize - (job.Len - job.Offset),
				})
			}
		}
	}

	return nil
}

func (p *MachineMigrationTask_StorageProcessor) newMirrorOpts(srcName, dstName string) *qemu_types.DriveMirrorOptions {
	opts := qemu_types.DriveMirrorOptions{
		JobID:  fmt.Sprintf("migr_%s", srcName),
		Device: srcName,
		Target: fmt.Sprintf("nbd:%s:%d:exportname=%s", p.t.dstServerAddr.String(), int(p.t.requisites.NBDPort), dstName),
		Format: "nbd",
		Sync:   "full",
		Mode:   "existing",
	}

	if p.t.vm.R.GetQemuVersion().Int() >= 60000 { // >= 6.x.x
		opts.CopyMode = "write-blocking"
	}

	return &opts
}
