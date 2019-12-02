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
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	qmp "github.com/0xef53/go-qmp"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	qt "github.com/0xef53/kvmrun/pkg/qemu/types"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
	"github.com/0xef53/kvmrun/pkg/runsv"
)

//
// POOL
//

type MigrationPool struct {
	mu    sync.Mutex
	table map[string]*Migration
}

func NewMigrationPool() *MigrationPool {
	p := MigrationPool{}
	p.table = make(map[string]*Migration)
	return &p
}

func (p *MigrationPool) Get(vmname string) (*Migration, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if m, found := p.table[vmname]; found {
		return m, nil
	}

	m := &Migration{vmname: vmname}

	p.table[vmname] = m

	return m, nil
}

func (p *MigrationPool) Exists(vmname string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, found := p.table[vmname]; found {
		return true
	}

	return false
}

func (p *MigrationPool) release(vmname string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if m, found := p.table[vmname]; found {
		// Cancel the existing process ...
		m.Cancel()
		// ... and wait for it to be done
		<-m.released

		delete(p.table, vmname)
	}
}

func (p *MigrationPool) Release(vmname string) {
	p.release(vmname)
}

//
// STAT
//

type StatUpdate struct {
	Kind        string
	Name        string
	Total       uint64
	Remaining   uint64
	Transferred uint64
	Speed       uint
	NewStatus   string
}

//
// OPTS
//

type MigrationOpts struct {
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

//
// MAIN
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

type Migration struct {
	mu sync.Mutex

	vmname string
	opts   *MigrationOpts

	stat          *rpccommon.MigrationStat
	statPipe      chan StatUpdate
	terminateStat chan struct{}
	statCompleted chan struct{}

	completed bool
	cancel    context.CancelFunc
	released  chan struct{}

	err error
}

func (m *Migration) printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf("[migration:"+m.vmname+"] "+format+"\n", a...)
}

func (m *Migration) debugf(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(DebugWriter, "[migration:"+m.vmname+"] DEBUG: "+format+"\n", a...)
}

// Err returns the last migration error
func (m *Migration) Err() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.err
}

func (m *Migration) Cancel() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel == nil {
		return fmt.Errorf("migration process is not running")
	}

	m.cancel()

	return nil
}

func (m *Migration) Inmigrate() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel == nil {
		return false
	}

	return true
}

func (m *Migration) Stat() *rpccommon.MigrationStat {
	return m.stat
}

func (m *Migration) Start(opts *MigrationOpts) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		return fmt.Errorf("migration process is already running")
	}

	m.opts = opts

	m.released = make(chan struct{})

	m.statPipe = make(chan StatUpdate, 10)
	m.terminateStat = make(chan struct{})
	m.statCompleted = make(chan struct{})

	// Make the initial statistics structure
	m.stat = &rpccommon.MigrationStat{
		DstServer: opts.DstServer,
		Status:    "starting",
		Qemu:      new(rpccommon.StatInfo),
		Disks:     make(map[string]*rpccommon.StatInfo),
	}
	for _, d := range opts.Disks {
		m.stat.Disks[d.Path] = &rpccommon.StatInfo{Total: d.QemuVirtualSize}
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	// Main migration process
	go func() {
		var err error

		m.printf("Starting")

		defer func() {
			m.mu.Lock()
			m.cancel = nil
			m.err = err
			close(m.released)
			m.mu.Unlock()
		}()

		switch err = m.migrate(ctx); {
		case err == nil:
			m.completed = true
			m.statPipe <- StatUpdate{Kind: "status", NewStatus: "completed"}
		case IsInterruptedError(err):
			m.statPipe <- StatUpdate{Kind: "status", NewStatus: "interrupted"}
			m.printf("Interrupted by the CANCEL command")
		default:
			m.statPipe <- StatUpdate{Kind: "status", NewStatus: "failed"}
			m.printf("Fatal error: %s", err)
		}
		m.err = err

		// Wait until the stat collector is completed
		close(m.terminateStat)
		<-m.statCompleted

		// Forced shutdown when migration is successful
		if err == nil && m.completed {
			if err := ioutil.WriteFile(filepath.Join(kvmrun.VMCONFDIR, m.vmname, "down"), []byte{}, 0644); err != nil {
				m.printf(err.Error())
			}
			if err := runsv.SendSignal(m.vmname, "x"); err != nil {
				m.printf(err.Error())
			}

			m.printf("Successfully completed")
		}
	}()

	// The stat file could have remained since the previous attempt. Remove it
	os.Remove(filepath.Join(kvmrun.VMCONFDIR, m.vmname, "supervise/migration_stat"))

	// Stat collecting
	go func() {
		defer close(m.statCompleted)

		printDebugStat := func(name string, st *rpccommon.StatInfo) {
			m.debugf(
				"%s: total=%d, transferred=%d, remaining=%d, percent=%d, speed=%dmbit/s",
				name,
				st.Total,
				st.Transferred,
				st.Remaining,
				st.Percent,
				st.Speed,
			)
		}

		for {
			select {
			case <-m.terminateStat:
				if m.err != nil {
					m.stat.Desc = m.err.Error()
				}
				jb, err := json.Marshal(m.stat)
				if err != nil {
					m.printf(err.Error())
				}
				if err := ioutil.WriteFile(filepath.Join(kvmrun.VMCONFDIR, m.vmname, "supervise/migration_stat"), jb, 0644); err != nil {
					m.printf(err.Error())
				}
				return
			case x := <-m.statPipe:
				switch x.Kind {
				case "status":
					m.stat.Status = x.NewStatus
				case "qemu":
					if x.Total == 0 {
						break
					}
					st := &rpccommon.StatInfo{}
					st.Total = x.Total
					st.Remaining = x.Remaining
					st.Transferred = x.Transferred
					st.Percent = uint(x.Transferred * 100 / x.Total)
					st.Speed = x.Speed
					m.stat.Qemu = st
					printDebugStat("qemu_vmstate", st)
				case "disk":
					st := &rpccommon.StatInfo{}
					st.Total = m.stat.Disks[x.Name].Total // copy from previous
					st.Remaining = x.Remaining
					st.Transferred = x.Transferred
					st.Percent = uint(x.Transferred * 100 / st.Total)
					st.Speed = uint(((x.Transferred - m.stat.Disks[x.Name].Transferred) * 8) / 1 >> (10 * 2)) // mbit/s
					m.stat.Disks[x.Name] = st
					printDebugStat(x.Name, st)
				}
			}
		}
	}()

	return nil
}

func (m *Migration) migrate(ctx context.Context) error {
	m.debugf("migrate(): main process started")

	var success bool

	defer func() {
		if success {
			return
		}
		m.debugf("migrate(): something went wrong. Removing the scraps")
		m.cleanWhenInterrupted()
		m.cleanDstWhenInterrupted()
	}()

	m.statPipe <- StatUpdate{Kind: "status", NewStatus: "inmigrate"}

	// Check params such as the size and the type
	// of block devices on the destination server
	if len(m.opts.Disks) > 0 {
		if err := m.checkDstDisks(); err != nil {
			return err
		}
	}

	m.debugf("migrate(): starting the incoming instance on destination host")
	// Start the incoming instance on the destination server
	if port, err := m.startDstIncomingInstance(); err == nil {
		m.opts.IncomingPort = port
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
	if len(m.opts.Disks) > 0 {
		m.debugf("migrate(): starting the NBD server on destination host")
		// Start NBD server on the destination server and export devices
		if port, err := m.startDstNBDServer(); err == nil {
			m.opts.NBDPort = port
		} else {
			return err
		}

		group.Go(func() error { return m.mirrorDisks(ctx, allDisksReady, vmstateMigrated) })
	}

	// Start the migration of virtual machine state
	group.Go(func() error { return m.migrateVMState(ctx, allDisksReady, vmstateMigrated) })

	// and wait for their completion
	if err := group.Wait(); err != nil {
		return err
	}

	if len(m.opts.Disks) > 0 {
		// Stop the NBD server on destination server
		m.debugf("migrate(): stopping the NBD server on destination host")
		if err := m.stopDstNBDServer(); err != nil {
			return err
		}
	}

	// Send the CONT signal to make sure that the remote instance is alive
	if err := m.sendDstCont(); err != nil {
		m.printf("Failed to send CONT signal: %s", err)
	}

	success = true

	m.debugf("migrate(): completed")

	return nil
}

func (m *Migration) migrateVMState(ctx context.Context, allDisksReady, vmstateMigrated chan struct{}) error {
	m.debugf("migrateVMState(): starting QEMU memory + state migration")

	if len(m.opts.Disks) > 0 {
		m.debugf("migrateVMState(): waiting for disks synchronization ...")
		select {
		case <-ctx.Done():
			return &InterruptedError{}
		case <-allDisksReady:
			break
		}
	}
	m.debugf("migrateVMState(): disks are synchronized")

	m.debugf("migrateVMState(): running QMP command: migrate-set-capabilities")
	// Capabilities && Parameters
	capsArgs := struct {
		Capabilities []qt.MigrationCapabilityStatus `json:"capabilities"`
	}{
		Capabilities: []qt.MigrationCapabilityStatus{
			qt.MigrationCapabilityStatus{"xbzrle", true},
			qt.MigrationCapabilityStatus{"auto-converge", true},
			qt.MigrationCapabilityStatus{"compress", false},
			qt.MigrationCapabilityStatus{"block", false},
		},
	}
	if err := QPool.Run(m.vmname, qmp.Command{"migrate-set-capabilities", &capsArgs}, nil); err != nil {
		return err
	}

	m.debugf("migrateVMState(): running QMP command: migrate-set-parameters")
	paramsArgs := qt.MigrateSetParameters{
		MaxBandwidth:    8589934592,
		XbzrleCacheSize: 536870912,
	}
	if err := QPool.Run(m.vmname, qmp.Command{"migrate-set-parameters", &paramsArgs}, nil); err != nil {
		return err
	}

	m.debugf("migrateVMState(): running QMP command: migrate; waiting for QEMU migration ...")
	// Run
	args := struct {
		URI string `json:"uri"`
	}{
		URI: fmt.Sprintf("tcp:%s:%d", m.opts.DstServerIPs[0], m.opts.IncomingPort),
	}

	if err := QPool.Run(m.vmname, qmp.Command{"migrate", &args}, nil); err != nil {
		return err
	}

loop:
	for {
		select {
		case <-ctx.Done():
			return &InterruptedError{}
		default:
		}

		mi := &qt.MigrationInfo{}

		if err := QPool.Run(m.vmname, qmp.Command{"query-migrate", nil}, mi); err != nil {
			return err
		}

		switch mi.Status {
		case "active", "postcopy-active", "completed":
			m.statPipe <- StatUpdate{
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
			fmt.Println("DEBUG: latest vmstate:", mi)
			break loop
		case "failed":
			return fmt.Errorf("QEMU migration failed: %s", mi.ErrDesc)
		case "cancelled":
			return fmt.Errorf("QEMU migration cancelled by QMP command")
		}

		time.Sleep(time.Second * 1)
	}

	close(vmstateMigrated)

	m.debugf("migrateVMState(): completed")

	return nil
}

func (m *Migration) mirrorDisks(ctx context.Context, allDisksReady, vmstateMigrated chan struct{}) error {
	m.debugf("mirrorDisks(): starting disks mirroring process")

	errJobNotFound := errors.New("Job not found")

	getJob := func(jobID string) (*qt.BlockJobInfo, error) {
		jobs := make([]*qt.BlockJobInfo, 0, len(m.opts.Disks))
		if err := QPool.Run(m.vmname, qmp.Command{"query-block-jobs", nil}, &jobs); err != nil {
			return nil, err
		}
		for _, j := range jobs {
			if j.Device == jobID {
				return j, nil
			}
		}
		return nil, errJobNotFound
	}

	waitForReady := func(ctx context.Context, d *kvmrun.Disk) error {
		jobID := "migr_" + d.BaseName()

		// Stat will be available after the job status changes to running
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := QPool.WaitJobStatusChangeEvent(m.vmname, timeoutCtx, jobID, "running", uint64(time.Now().Unix())); err != nil {
			return err
		}

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return &InterruptedError{}
			case <-ticker.C:
			}

			job, err := getJob(jobID)
			if err != nil {
				// No errors should be here, even errJobNotfound
				return err
			}

			m.statPipe <- StatUpdate{
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
	for _, d := range m.opts.Disks {
		m.debugf("mirrorDisks(): running QMP command: drive-mirror; name=%s, remote_addr=%s:%d", d.BaseName(), m.opts.DstServerIPs[0].String(), m.opts.NBDPort)
		d := d // shadow to be captured by closure
		dstName := d.BaseName()
		if ovrd, ok := m.opts.Overrides.Disks[d.Path]; ok {
			if _d, err := kvmrun.NewDisk(ovrd); err == nil {
				dstName = _d.BaseName()
			} else {
				return err
			}
		}
		args := newQemuDriveMirrorOpts(m.opts.DstServerIPs[0].String(), m.opts.NBDPort, d.BaseName(), dstName)
		if err := QPool.Run(m.vmname, qmp.Command{"drive-mirror", &args}, nil); err != nil {
			return err
		}
		group1.Go(func() error { return waitForReady(ctx1, &d) })
	}

	m.debugf("mirrorDisks(): waiting for disks synchronization ...")
	if err := group1.Wait(); err != nil {
		return err
	}
	m.debugf("mirrorDisks(): disks are synchronized")

	// All disks are ready. Notify migrateVMState() goroutine about this
	close(allDisksReady)

	m.debugf("mirrorDisks(): channel allDiskReady is closed now")

	// Stat update and wait for migrateVMState() is completed
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			return &InterruptedError{}
		case <-vmstateMigrated:
			ticker.Stop()
			break loop
		case <-ticker.C:
		}

		for _, d := range m.opts.Disks {
			job, err := getJob("migr_" + d.BaseName())
			if err != nil {
				return err
			}

			m.statPipe <- StatUpdate{
				Kind:        "disk",
				Name:        d.Path,
				Total:       d.QemuVirtualSize,
				Remaining:   job.Len - job.Offset,
				Transferred: d.QemuVirtualSize - (job.Len - job.Offset),
			}
		}
	}

	m.debugf("mirrorDisks(): QEMU migration is completed. Waiting for total disks synchronization ...")

	waitForCompleted := func(ctx context.Context, d *kvmrun.Disk) error {
		jobID := "migr_" + d.BaseName()

		ts := time.Now()

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return &InterruptedError{}
			case <-ticker.C:
			}

			job, err := getJob(jobID)
			if err != nil && err != errJobNotFound {
				return fmt.Errorf("Failed to complete disk mirroring for %s: %s", d.BaseName(), err)
			}
			// Ok, job completed
			if job == nil {
				if _, found, err := QPool.FindBlockJobErrorEvent(m.vmname, jobID, uint64(ts.Unix())); err == nil {
					if found {
						return fmt.Errorf("Errors detected during disk mirroring: %s", d.BaseName())
					}
				} else {
					return fmt.Errorf("FindBlockJobErrorEvent failed: %s: %s", d.BaseName(), err)
				}
				if _, found, err := QPool.FindBlockJobCompletedEvent(m.vmname, jobID, uint64(ts.Unix())); err == nil {
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

	m.debugf("mirrorDisks(): finishing all drive-mirror processes ...")

	group2, ctx2 := errgroup.WithContext(ctx)

	// Stop the mirroring
	for _, d := range m.opts.Disks {
		m.debugf("mirrorDisks(): running QMP command: block-job-complete; name=%s", d.BaseName())
		d := d // shadow for closure
		jobID := struct {
			Device string `json:"device"`
		}{
			Device: "migr_" + d.BaseName(),
		}
		if err := QPool.Run(m.vmname, qmp.Command{"block-job-complete", &jobID}, nil); err != nil {
			// Non-fatal error
			m.printf(err.Error())
			continue
		}
		group2.Go(func() error { return waitForCompleted(ctx2, &d) })
	}

	switch err := group2.Wait(); {
	case err == nil:
	case IsInterruptedError(err):
		return err
	default:
		// The migration is actually done,
		// so we don't care about these errors
		m.printf(err.Error())
	}

	m.debugf("mirrorDisks(): completed")

	return nil
}

func (m *Migration) checkDstDisks() error {
	disks := make(map[string]uint64)
	for _, d := range m.opts.Disks {
		if ovrd, ok := m.opts.Overrides.Disks[d.Path]; ok {
			disks[ovrd] = d.QemuVirtualSize
		} else {
			disks[d.Path] = d.QemuVirtualSize
		}
	}

	req := rpccommon.CheckDisksRequest{
		Disks: disks,
	}

	if err := RPCClient.Request(m.opts.DstServer, "RPC.CheckDisks", &req, nil); err != nil {
		return fmt.Errorf("failed to check block devices: %s", err)
	}

	return nil
}

func (m *Migration) startDstIncomingInstance() (int, error) {
	var port int

	req := rpccommon.NewManifestInstanceRequest{
		Name:     m.vmname,
		Manifest: m.opts.Manifest,
	}

	// Run file
	if l, err := os.Readlink(filepath.Join(kvmrun.VMCONFDIR, m.vmname, "run")); err == nil {
		req.Launcher = l
	}

	// Finish file
	if l, err := os.Readlink(filepath.Join(kvmrun.VMCONFDIR, m.vmname, "finish")); err == nil {
		req.Finisher = l
	}

	// Some extra files that may contain additional
	// configuration such as network settings.
	if ff, err := getExtraFiles(m.vmname); err == nil {
		req.ExtraFiles = ff
	} else {
		return 0, err
	}

	if err := RPCClient.Request(m.opts.DstServer, "RPC.StartIncomingInstance", &req, &port); err != nil {
		return 0, fmt.Errorf("failed to start incoming instance: %s", err)
	}

	return port, nil
}

func (m *Migration) startDstNBDServer() (int, error) {
	var port int

	disks := make([]string, 0, len(m.opts.Disks))
	for _, d := range m.opts.Disks {
		if ovrd, ok := m.opts.Overrides.Disks[d.Path]; ok {
			disks = append(disks, ovrd)
		} else {
			disks = append(disks, d.Path)
		}
	}

	req := rpccommon.InstanceRequest{
		Name: m.vmname,
		Data: &rpccommon.NBDParams{
			ListenAddr: m.opts.DstServerIPs[0].String(),
			Disks:      disks,
		},
	}

	if err := RPCClient.Request(m.opts.DstServer, "RPC.StartNBDServer", &req, &port); err != nil {
		return 0, fmt.Errorf("failed to start NBD server: %s", err)
	}

	return port, nil
}

func (m *Migration) stopDstNBDServer() error {
	req := rpccommon.VMNameRequest{
		Name: m.vmname,
	}

	if err := RPCClient.Request(m.opts.DstServer, "RPC.StopNBDServer", &req, nil); err != nil {
		return fmt.Errorf("failed to stop NBD server: %s", err)
	}

	return nil
}

func (m *Migration) sendDstCont() error {
	req := rpccommon.VMNameRequest{
		Name: m.vmname,
	}

	if err := RPCClient.Request(m.opts.DstServer, "RPC.SendCont", &req, nil); err != nil {
		return fmt.Errorf("failed to send CONT: %s", err)
	}

	return nil
}

//
// PURGE FUNCTIONS
//

func (m *Migration) cleanWhenInterrupted() {
	for _, d := range m.opts.Disks {
		cancelOpts := struct {
			Device string `json:"device"`
			Force  bool   `json:"force,omitempty"`
		}{
			Device: "migr_" + d.BaseName(),
			Force:  true,
		}
		if err := QPool.Run(m.vmname, qmp.Command{"block-job-cancel", &cancelOpts}, nil); err != nil {
			// Non-fatal error. Just printing
			m.printf("Forced block-job-cancel failed: %s", err)
		}
	}

	if err := QPool.Run(m.vmname, qmp.Command{"migrate_cancel", nil}, nil); err != nil {
		// Non-fatal error. Just printing
		m.printf("Forced migrate_cancel failed: %s", err)
	}
}

func (m *Migration) cleanDstWhenInterrupted() {
	req := rpccommon.InstanceRequest{
		Name: m.vmname,
	}

	if err := RPCClient.Request(m.opts.DstServer, "RPC.RemoveConfInstance", &req, nil); err != nil {
		// Non-fatal error. Just printing
		m.printf("Failed to remove destination configuration: %s", err)
	}
}

//
//  AUXILIARY FUNCTIONS
//

// getExtraFiles returns a map including the contents of some extra files
// placed in the virtual machine directory.
// These files may contain additional configuration such as network settings
// (config_network).
func getExtraFiles(vmname string) (map[string][]byte, error) {
	r := regexp.MustCompile(`^(extra|comment|config_[[:alnum:]]*|[\.\_[:alnum:]]*_config)$`)

	vmdir := filepath.Join(kvmrun.VMCONFDIR, vmname)

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
