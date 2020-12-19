package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	qt "github.com/0xef53/kvmrun/pkg/qemu/types"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	qmp "github.com/0xef53/go-qmp/v2"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type StatUpdate struct {
	Kind        string
	Name        string
	Total       uint64
	Remaining   uint64
	Transferred uint64
	Speed       uint
	NewStatus   string
	ErrDesc     string
}

type MigrationTaskOpts struct {
	VMName       string
	Manifest     []byte
	Disks        kvmrun.Disks
	DstServer    string
	DstServerIPs []net.IP
	IncomingPort int
	NBDPort      int
	Overrides    struct {
		Disks map[string]string
	}
}

type MigrationTask struct {
	task

	opts *MigrationTaskOpts

	stat          *rpccommon.MigrationTaskStat
	statPipe      chan StatUpdate
	terminateStat chan struct{}
	statCompleted chan struct{}
}

func (t *MigrationTask) Stat() *rpccommon.MigrationTaskStat {
	t.mu.Lock()
	defer t.mu.Unlock()

	x := *t.stat

	qemu := *t.stat.Qemu

	disks := make(map[string]*rpccommon.StatInfo, len(t.stat.Disks))
	for k, v := range t.stat.Disks {
		tmp := *v
		disks[k] = &tmp
	}

	x.Qemu = &qemu
	x.Disks = disks

	return &x
}

func (t *MigrationTask) updateStat(u *StatUpdate) {
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
	case "error":
		t.stat.Desc = u.ErrDesc
	case "qemu":
		if u.Total == 0 {
			break
		}
		st := &rpccommon.StatInfo{}
		st.Total = u.Total
		st.Remaining = u.Remaining
		st.Transferred = u.Transferred
		st.Percent = uint(u.Transferred * 100 / u.Total)
		st.Speed = u.Speed
		t.stat.Qemu = st
		printDebugStat("qemu_vmstate", st)
	case "disk":
		st := &rpccommon.StatInfo{}
		st.Total = t.stat.Disks[u.Name].Total // copy from previous
		st.Remaining = u.Remaining
		st.Transferred = u.Transferred
		st.Percent = uint(u.Transferred * 100 / st.Total)
		st.Speed = uint(((u.Transferred - t.stat.Disks[u.Name].Transferred) * 8) / 1 >> (10 * 2)) // mbit/s
		t.stat.Disks[u.Name] = st
		printDebugStat(u.Name, st)
	}

}

func (t *MigrationTask) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel != nil {
		return fmt.Errorf("already running")
	}

	t.released = make(chan struct{})

	t.statPipe = make(chan StatUpdate, 10)
	t.terminateStat = make(chan struct{})
	t.statCompleted = make(chan struct{})

	// Make the initial statistics structure
	t.stat = &rpccommon.MigrationTaskStat{
		DstServer: t.opts.DstServer,
		Status:    "starting",
		Qemu:      new(rpccommon.StatInfo),
		Disks:     make(map[string]*rpccommon.StatInfo),
	}
	for _, d := range t.opts.Disks {
		t.stat.Disks[d.Path] = &rpccommon.StatInfo{Total: d.QemuVirtualSize}
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	// Main migration process
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

		switch err = t.migrate(ctx); {
		case err == nil:
			t.completed = true
			t.statPipe <- StatUpdate{Kind: "status", NewStatus: "completed"}
		case IsTaskInterruptedError(err):
			t.statPipe <- StatUpdate{Kind: "status", NewStatus: "interrupted"}
			t.statPipe <- StatUpdate{Kind: "error", ErrDesc: err.Error()}
			t.logger.Warn("Interrupted by the CANCEL command")
		default:
			t.statPipe <- StatUpdate{Kind: "status", NewStatus: "failed"}
			t.statPipe <- StatUpdate{Kind: "error", ErrDesc: err.Error()}
			t.logger.Errorf("Fatal error: %s", err)
		}

		// Wait until the stat collector is completed
		close(t.terminateStat)
		<-t.statCompleted

		// Forced shutdown when migration is successful
		if err == nil && t.completed {
			if err := t.mon.Run(t.opts.VMName, qmp.Command{"quit", nil}, nil); err != nil {
				t.logger.Error(err.Error())
			}

			t.logger.Info("Successfully completed")
		}
	}()

	// The stat file could have remained since the previous attempt. Remove it
	os.Remove(filepath.Join(kvmrun.CONFDIR, t.opts.VMName, ".runtime/migration_stat"))

	// Stat collecting
	go func() {
		defer close(t.statCompleted)

		for {
			select {
			case <-t.terminateStat:
				b, err := json.Marshal(t.Stat())
				if err != nil {
					t.logger.Error(err.Error())
				}
				if err := ioutil.WriteFile(filepath.Join(kvmrun.CONFDIR, t.opts.VMName, ".runtime/migration_stat"), b, 0644); err != nil {
					t.logger.Error(err.Error())
				}
				return
			case x := <-t.statPipe:
				t.updateStat(&x)
			}
		}
	}()

	return nil
}

func (t *MigrationTask) migrate(ctx context.Context) error {
	t.logger.Debug("migrate(): main process started")

	var success bool

	defer func() {
		if success {
			return
		}
		t.logger.Debug("migrate(): something went wrong. Removing the scraps")
		t.cleanWhenInterrupted()
		t.cleanDstWhenInterrupted()
	}()

	t.statPipe <- StatUpdate{Kind: "status", NewStatus: "inmigrate"}

	// Check params such as the size and the type
	// of block devices on the destination server
	if len(t.opts.Disks) > 0 {
		if err := t.checkDstDisks(); err != nil {
			return err
		}
	}

	t.logger.Debug("migrate(): starting the incoming instance on destination host")
	// Start the incoming instance on the destination server
	if port, err := t.startDstIncomingInstance(); err == nil {
		t.opts.IncomingPort = port
	} else {
		return err
	}

	// The migration of virtual machine state and its disks
	// are running in the parallel goroutines.
	// These variables are to syncronize the states between two goroutines
	allDisksReady := make(chan struct{})
	vmstateMigrated := make(chan struct{})

	group, ctx := errgroup.WithContext(ctx)

	// Start the NBD server and disks mirroring
	if len(t.opts.Disks) > 0 {
		t.logger.Debug("migrate(): starting the NBD server on destination host")
		// Start NBD server on the destination server and export devices
		if port, err := t.startDstNBDServer(); err == nil {
			t.opts.NBDPort = port
		} else {
			return err
		}

		group.Go(func() error { return t.mirrorDisks(ctx, allDisksReady, vmstateMigrated) })
	}

	// Start the migration of virtual machine state
	group.Go(func() error { return t.migrateVMState(ctx, allDisksReady, vmstateMigrated) })

	// and wait for their completion
	if err := group.Wait(); err != nil {
		return err
	}

	if len(t.opts.Disks) > 0 {
		// Stop the NBD server on destination server
		t.logger.Debug("migrate(): stopping the NBD server on destination host")
		if err := t.stopDstNBDServer(); err != nil {
			return err
		}
	}

	// Send the CONT signal to make sure that the remote instance is alive
	if err := t.sendDstCont(); err != nil {
		t.logger.Warnf("Failed to send CONT signal: %s", err)
	}

	success = true

	t.logger.Debug("migrate(): completed")

	return nil
}

func (t *MigrationTask) migrateVMState(ctx context.Context, allDisksReady, vmstateMigrated chan struct{}) error {
	t.logger.Debug("migrateVMState(): starting QEMU memory + state migration")

	if len(t.opts.Disks) > 0 {
		t.logger.Debug("migrateVMState(): waiting for disks synchronization ...")
		select {
		case <-ctx.Done():
			return &TaskInterruptedError{}
		case <-allDisksReady:
			break
		}
	}
	t.logger.Debug("migrateVMState(): disks are synchronized")

	t.logger.Debug("migrateVMState(): running QMP command: migrate-set-capabilities")
	// Capabilities && Parameters
	capsArgs := struct {
		Capabilities []qt.MigrationCapabilityStatus `json:"capabilities"`
	}{
		Capabilities: []qt.MigrationCapabilityStatus{
			qt.MigrationCapabilityStatus{"xbzrle", true},
			qt.MigrationCapabilityStatus{"auto-converge", true},
			qt.MigrationCapabilityStatus{"compress", false},
			qt.MigrationCapabilityStatus{"block", false},
			qt.MigrationCapabilityStatus{"dirty-bitmaps", true},
		},
	}
	if err := t.mon.Run(t.opts.VMName, qmp.Command{"migrate-set-capabilities", &capsArgs}, nil); err != nil {
		return err
	}

	t.logger.Debug("migrateVMState(): running QMP command: migrate-set-parameters")
	paramsArgs := qt.MigrateSetParameters{
		MaxBandwidth:    8589934592,
		XbzrleCacheSize: 536870912,
	}
	if err := t.mon.Run(t.opts.VMName, qmp.Command{"migrate-set-parameters", &paramsArgs}, nil); err != nil {
		return err
	}

	t.logger.Debug("migrateVMState(): running QMP command: migrate; waiting for QEMU migration ...")
	// Run
	args := struct {
		URI string `json:"uri"`
	}{
		URI: fmt.Sprintf("tcp:%s:%d", t.opts.DstServerIPs[0], t.opts.IncomingPort),
	}

	if err := t.mon.Run(t.opts.VMName, qmp.Command{"migrate", &args}, nil); err != nil {
		return err
	}

loop:
	for {
		select {
		case <-ctx.Done():
			return &TaskInterruptedError{}
		default:
		}

		mi := &qt.MigrationInfo{}

		if err := t.mon.Run(t.opts.VMName, qmp.Command{"query-migrate", nil}, mi); err != nil {
			return err
		}

		switch mi.Status {
		case "active", "postcopy-active", "completed":
			t.statPipe <- StatUpdate{
				Kind:        "qemu",
				Total:       mi.Ram.Total,
				Remaining:   mi.Ram.Remaining,
				Transferred: mi.Ram.Total - mi.Ram.Remaining,
				Speed:       uint(mi.Ram.Speed),
			}
		}

		switch mi.Status {
		case "active", "postcopy-active":
		case "completed":
			break loop
		case "failed":
			return fmt.Errorf("QEMU migration failed: %s", mi.ErrDesc)
		case "cancelled":
			return fmt.Errorf("QEMU migration cancelled by QMP command")
		}

		time.Sleep(time.Second * 1)
	}

	close(vmstateMigrated)

	t.logger.Debug("migrateVMState(): completed")

	return nil
}

func (t *MigrationTask) mirrorDisks(ctx context.Context, allDisksReady, vmstateMigrated chan struct{}) error {
	t.logger.Debug("mirrorDisks(): starting disks mirroring process")

	errJobNotFound := errors.New("Job not found")

	getJob := func(jobID string) (*qt.BlockJobInfo, error) {
		jobs := make([]*qt.BlockJobInfo, 0, len(t.opts.Disks))
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

	waitForReady := func(ctx context.Context, ts time.Time, d *kvmrun.Disk) error {
		jobID := "migr_" + d.BaseName()

		// Stat will be available after the job status changes to running
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := t.mon.WaitJobStatusChangeEvent(t.opts.VMName, timeoutCtx, jobID, "running", uint64(ts.Unix())); err != nil {
			return err
		}

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return &TaskInterruptedError{}
			case <-ticker.C:
			}

			job, err := getJob(jobID)
			if err != nil {
				// No errors should be here, even errJobNotfound
				return err
			}

			t.statPipe <- StatUpdate{
				Kind:        "disk",
				Name:        d.Path,
				Total:       d.QemuVirtualSize,
				Remaining:   job.Len - job.Offset,
				Transferred: d.QemuVirtualSize - (job.Len - job.Offset),
			}

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
	for _, d := range t.opts.Disks {
		t.logger.Debugf("mirrorDisks(): running QMP command: drive-mirror; name=%s, remote_addr=%s:%d", d.BaseName(), t.opts.DstServerIPs[0].String(), t.opts.NBDPort)
		d := d // shadow to be captured by closure
		dstName := d.BaseName()
		if ovrd, ok := t.opts.Overrides.Disks[d.Path]; ok {
			if _d, err := kvmrun.NewDisk(ovrd); err == nil {
				dstName = _d.BaseName()
			} else {
				return err
			}
		}
		ts := time.Now()
		args := newQemuDriveMirrorOpts(t.opts.DstServerIPs[0].String(), t.opts.NBDPort, d.BaseName(), dstName)
		if err := t.mon.Run(t.opts.VMName, qmp.Command{"drive-mirror", &args}, nil); err != nil {
			return err
		}
		group1.Go(func() error { return waitForReady(ctx1, ts, &d) })
	}

	t.logger.Debug("mirrorDisks(): waiting for disks synchronization ...")
	if err := group1.Wait(); err != nil {
		return err
	}
	t.logger.Debug("mirrorDisks(): disks are synchronized")

	// All disks are ready. Notify migrateVMState() goroutine about this
	close(allDisksReady)

	t.logger.Debug("mirrorDisks(): channel allDiskReady is closed now")

	// Stat update and wait for migrateVMState() is completed
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			return &TaskInterruptedError{}
		case <-vmstateMigrated:
			ticker.Stop()
			break loop
		case <-ticker.C:
		}

		for _, d := range t.opts.Disks {
			job, err := getJob("migr_" + d.BaseName())
			if err != nil {
				return err
			}

			t.statPipe <- StatUpdate{
				Kind:        "disk",
				Name:        d.Path,
				Total:       d.QemuVirtualSize,
				Remaining:   job.Len - job.Offset,
				Transferred: d.QemuVirtualSize - (job.Len - job.Offset),
			}
		}
	}

	t.logger.Debug("mirrorDisks(): QEMU migration is completed. Waiting for total disks synchronization ...")

	waitForCompleted := func(ctx context.Context, ts time.Time, d *kvmrun.Disk) error {
		jobID := "migr_" + d.BaseName()

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return &TaskInterruptedError{}
			case <-ticker.C:
			}

			job, err := getJob(jobID)
			if err != nil && err != errJobNotFound {
				return fmt.Errorf("Failed to complete disk mirroring for %s: %s", d.BaseName(), err)
			}
			// Ok, job completed
			if job == nil {
				if _, found, err := t.mon.FindBlockJobErrorEvent(t.opts.VMName, jobID, uint64(ts.Unix())); err == nil {
					if found {
						return fmt.Errorf("Errors detected during disk mirroring: %s", d.BaseName())
					}
				} else {
					return fmt.Errorf("FindBlockJobErrorEvent failed: %s: %s", d.BaseName(), err)
				}
				if _, found, err := t.mon.FindBlockJobCompletedEvent(t.opts.VMName, jobID, uint64(ts.Unix())); err == nil {
					if !found {
						return fmt.Errorf("No completed event found: %s", d.BaseName())
					}
				} else {
					return fmt.Errorf("FindBlockJobCompletedEvent failed: %s: %s", d.BaseName(), err)
				}
				break
			}
		}
		return nil
	}

	t.logger.Debug("mirrorDisks(): finishing all drive-mirror processes ...")

	group2, ctx2 := errgroup.WithContext(ctx)

	// Stop the mirroring
	for _, d := range t.opts.Disks {
		t.logger.Debugf("mirrorDisks(): running QMP command: block-job-complete; name=%s", d.BaseName())
		d := d // shadow for closure
		jobID := struct {
			Device string `json:"device"`
		}{
			Device: "migr_" + d.BaseName(),
		}
		ts := time.Now()
		if err := t.mon.Run(t.opts.VMName, qmp.Command{"block-job-complete", &jobID}, nil); err != nil {
			// Non-fatal error
			t.logger.Error(err.Error())
			continue
		}
		group2.Go(func() error { return waitForCompleted(ctx2, ts, &d) })
	}

	switch err := group2.Wait(); {
	case err == nil:
	case IsTaskInterruptedError(err):
		return err
	default:
		// The migration is actually done,
		// so we don't care about these errors
		t.logger.Error(err.Error())
	}

	t.logger.Debug("mirrorDisks(): completed")

	return nil
}

func (t *MigrationTask) checkDstDisks() error {
	disks := make(map[string]uint64)
	for _, d := range t.opts.Disks {
		if ovrd, ok := t.opts.Overrides.Disks[d.Path]; ok {
			disks[ovrd] = d.QemuVirtualSize
		} else {
			disks[d.Path] = d.QemuVirtualSize
		}
	}

	req := rpccommon.CheckDisksRequest{
		Disks: disks,
	}

	if err := t.rpcClient.Request(t.opts.DstServer, "RPC.CheckDisks", &req, nil); err != nil {
		return fmt.Errorf("failed to check block devices: %s", err)
	}

	return nil
}

func (t *MigrationTask) startDstIncomingInstance() (int, error) {
	var port int

	req := rpccommon.NewManifestInstanceRequest{
		Name:     t.opts.VMName,
		Manifest: t.opts.Manifest,
	}

	// Some extra files that may contain additional
	// configuration such as network settings.
	if ff, err := getExtraFiles(t.opts.VMName); err == nil {
		req.ExtraFiles = ff
	} else {
		return 0, err
	}

	if err := t.rpcClient.Request(t.opts.DstServer, "RPC.StartIncomingInstance", &req, &port); err != nil {
		return 0, fmt.Errorf("failed to start incoming instance: %s", err)
	}

	return port, nil
}

func (t *MigrationTask) startDstNBDServer() (int, error) {
	var port int

	disks := make([]string, 0, len(t.opts.Disks))
	for _, d := range t.opts.Disks {
		if ovrd, ok := t.opts.Overrides.Disks[d.Path]; ok {
			disks = append(disks, ovrd)
		} else {
			disks = append(disks, d.Path)
		}
	}

	req := rpccommon.InstanceRequest{
		Name: t.opts.VMName,
		Data: &rpccommon.NBDParams{
			ListenAddr: t.opts.DstServerIPs[0].String(),
			Disks:      disks,
		},
	}

	if err := t.rpcClient.Request(t.opts.DstServer, "RPC.StartNBDServer", &req, &port); err != nil {
		return 0, fmt.Errorf("failed to start NBD server: %s", err)
	}

	return port, nil
}

func (t *MigrationTask) stopDstNBDServer() error {
	req := rpccommon.VMNameRequest{
		Name: t.opts.VMName,
	}

	if err := t.rpcClient.Request(t.opts.DstServer, "RPC.StopNBDServer", &req, nil); err != nil {
		return fmt.Errorf("failed to stop NBD server: %s", err)
	}

	return nil
}

func (t *MigrationTask) sendDstCont() error {
	req := rpccommon.VMNameRequest{
		Name: t.opts.VMName,
	}

	if err := t.rpcClient.Request(t.opts.DstServer, "RPC.SendCont", &req, nil); err != nil {
		return fmt.Errorf("failed to send CONT: %s", err)
	}

	return nil
}

//
// PURGE FUNCTIONS
//

func (t *MigrationTask) cleanWhenInterrupted() {
	for _, d := range t.opts.Disks {
		cancelOpts := struct {
			Device string `json:"device"`
			Force  bool   `json:"force,omitempty"`
		}{
			Device: "migr_" + d.BaseName(),
			Force:  true,
		}
		if err := t.mon.Run(t.opts.VMName, qmp.Command{"block-job-cancel", &cancelOpts}, nil); err != nil {
			// Non-fatal error. Just printing
			t.logger.Errorf("Forced block-job-cancel failed: %s", err)
		}
	}

	if err := t.mon.Run(t.opts.VMName, qmp.Command{"migrate_cancel", nil}, nil); err != nil {
		// Non-fatal error. Just printing
		t.logger.Errorf("Forced migrate_cancel failed: %s", err)
	}
}

func (t *MigrationTask) cleanDstWhenInterrupted() {
	req := rpccommon.InstanceRequest{
		Name: t.opts.VMName,
	}

	if err := t.rpcClient.Request(t.opts.DstServer, "RPC.RemoveConfInstance", &req, nil); err != nil {
		// Non-fatal error. Just printing
		t.logger.Errorf("Failed to remove destination configuration: %s", err)
	}
}

//
//  AUXILIARY FUNCTIONS
//

func newQemuDriveMirrorOpts(addr string, port int, srcName, dstName string) *qt.DriveMirrorOptions {
	return &qt.DriveMirrorOptions{
		JobID:  fmt.Sprintf("migr_%s", srcName),
		Device: srcName,
		Target: fmt.Sprintf("nbd:%s:%d:exportname=%s", addr, port, dstName),
		Format: "nbd",
		Sync:   "full",
		Mode:   "existing",
	}
}

// getExtraFiles returns a map including the contents of some extra files
// placed in the virtual machine directory.
// These files may contain additional configuration such as network settings
// (config_network).
func getExtraFiles(vmname string) (map[string][]byte, error) {
	r := regexp.MustCompile(`^(extra|comment|config_[[:alnum:]]*|[\.\_[:alnum:]]*_config)$`)

	vmdir := filepath.Join(kvmrun.CONFDIR, vmname)

	files, err := ioutil.ReadDir(vmdir)
	if err != nil {
		return nil, err
	}

	extraFiles := make(map[string][]byte)

	for _, f := range files {
		if !f.Mode().IsRegular() || f.Name() == "config" || !r.MatchString(f.Name()) {
			continue
		}
		c, err := ioutil.ReadFile(filepath.Join(vmdir, f.Name()))
		if err != nil {
			return nil, err
		}
		extraFiles[f.Name()] = c
	}

	return extraFiles, nil
}
