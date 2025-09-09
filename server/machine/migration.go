package machine

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/0xef53/kvmrun/internal/osuser"
	qemu_types "github.com/0xef53/kvmrun/internal/qemu/types"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	pb_machines "github.com/0xef53/kvmrun/api/services/machines/v2"
	pb_system "github.com/0xef53/kvmrun/api/services/system/v2"
	pb_tasks "github.com/0xef53/kvmrun/api/services/tasks/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"

	qmp "github.com/0xef53/go-qmp/v2"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type MigrationOptions struct {
	Disks       []string           `json:"disks"`
	Overrides   MigrationOverrides `json:"overrides"`
	CreateDisks bool               `json:"create_disks"`
	RemoveAfter bool               `json:"remove_after"`
}

func (o *MigrationOptions) Validate(strict bool) error {
	for idx, dname := range o.Disks {
		if len(strings.TrimSpace(dname)) == 0 {
			return fmt.Errorf("disk #%d has an empty name", idx)
		}
	}

	if err := o.Overrides.Validate(strict); err != nil {
		return err
	}

	return nil
}

type MigrationOverrides struct {
	Name      string            `json:"name"`
	Disks     map[string]string `json:"disks"`
	NetIfaces map[string]string `json:"net_ifaces"`
}

func (o *MigrationOverrides) Validate(_ bool) error {
	o.Name = strings.TrimSpace(o.Name)

	for key, value := range o.Disks {
		if len(strings.TrimSpace(key)) == 0 {
			return fmt.Errorf("empty key in 'disks' map")
		}

		if len(strings.TrimSpace(value)) == 0 {
			return fmt.Errorf("empty value in 'disks' map")
		}
	}

	for key, value := range o.NetIfaces {
		if len(strings.TrimSpace(key)) == 0 {
			return fmt.Errorf("empty key in 'net_ifaces' map")
		}

		if len(strings.TrimSpace(value)) == 0 {
			return fmt.Errorf("empty value in 'net_ifaces' map")
		}
	}

	return nil
}

func (s *Server) StartMigrationProcess(ctx context.Context, vmname, dstServer string, opts *MigrationOptions) (string, error) {
	if opts == nil {
		return "", fmt.Errorf("empty migration opts")
	}

	t := NewMachineMigrationTask(vmname, dstServer, opts)

	t.Server = s

	taskOpts := []task.TaskOption{
		server.WithUniqueLabel(vmname + "/migration"),
		server.WithGroupLabel(vmname),
		server.WithGroupLabel(vmname + "/long-running"),
	}

	tid, err := s.TaskStart(ctx, t, nil, taskOpts...)
	if err != nil {
		return "", fmt.Errorf("cannot start migration: %w", err)
	}

	return tid, nil
}

type MachineMigrationStatDetails struct {
	DstServer string
	VMState   *DataTransferStat
	Disks     map[string]*DataTransferStat
}

type MachineMigrationTask struct {
	*task.GenericTask
	*Server

	targets map[string]task.OperationMode

	// Arguments
	vmname    string
	dstServer string
	opts      *MigrationOptions

	// Do not set manually next fields !
	vm *kvmrun.Machine

	movingDisks   []*kvmrun.Disk
	dstServerAddr net.IP
	turnOffAfter  bool
	requisites    *pb_types.IncomingMigrationRequisites

	details *MachineMigrationStatDetails

	mu sync.Mutex
}

func NewMachineMigrationTask(vmname, dstServer string, opts *MigrationOptions) *MachineMigrationTask {
	return &MachineMigrationTask{
		GenericTask: new(task.GenericTask),

		targets:   server.BlockAnyOperations(vmname),
		vmname:    vmname,
		dstServer: dstServer,
		opts:      opts,
	}
}

func (t *MachineMigrationTask) Targets() map[string]task.OperationMode { return t.targets }

func (t *MachineMigrationTask) BeforeStart(_ interface{}) error {
	if t.opts == nil {
		return fmt.Errorf("empty migration opts")
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
	case kvmrun.StatePaused:
		// Machine will be turned off on the DST server
		t.turnOffAfter = true

		fallthrough
	case kvmrun.StateRunning:
		if t.vm.R == nil {
			return fmt.Errorf("unexpected error: QEMU instance not found")
		}
	default:
		return fmt.Errorf("incompatible machine state: %d", vmstate)
	}

	if ips, err := net.LookupIP(t.dstServer); err == nil {
		t.dstServerAddr = ips[0]
	} else {
		return err
	}

	t.movingDisks = make([]*kvmrun.Disk, 0, len(t.opts.Disks))

	// t.opts.Disks may contain short names and that's OK
	for _, dname := range t.opts.Disks {
		d := t.vm.R.DiskGet(dname)
		if d == nil {
			return &kvmrun.NotConnectedError{Source: "instance_qemu", Object: dname}
		}

		t.movingDisks = append(t.movingDisks, d)
	}

	// The keys of t.opts.Overrides.Disks may be presented as short names
	// and must be converted to fully qualified notation.
	// The values must be specified as fully qualified names as well.
	if len(t.opts.Overrides.Disks) > 0 {
		valid := make(map[string]string)

		for _, d := range t.movingDisks {
			if ovrd, ok := t.opts.Overrides.Disks[d.Path]; ok {
				valid[d.Path] = ovrd
			} else if ovrd, ok := t.opts.Overrides.Disks[d.Backend.BaseName()]; ok {
				valid[d.Path] = ovrd
			}
		}

		t.opts.Overrides.Disks = valid
	}

	// Incoming migration process on DST server
	if r, err := t.requestIncomingMigration(); err == nil {
		t.requisites = r
	} else {
		return fmt.Errorf("failed to request incoming migration: %w", err)
	}

	// Init stat fields
	t.details = &MachineMigrationStatDetails{
		DstServer: t.dstServer,
		VMState:   new(DataTransferStat),
		Disks:     make(map[string]*DataTransferStat),
	}
	for _, d := range t.movingDisks {
		t.details.Disks[d.Path] = &DataTransferStat{Total: d.QemuVirtualSize}
	}

	return nil
}

func (t *MachineMigrationTask) Stat() *task.TaskStat {
	t.mu.Lock()
	defer t.mu.Unlock()

	st := t.GenericTask.Stat()

	st.Details = t.details

	return st
}

func (t *MachineMigrationTask) updateStat(kind, name string, total, remain, sent uint64, speed int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch kind {
	case "vmstate":
		if total > 0 {
			t.details.VMState = &DataTransferStat{
				Total:       total,
				Remaining:   remain,
				Transferred: sent,
				Progress:    int(sent * 100 / total),
				Speed:       speed,
			}
		}
	case "disk":
		if total > 0 {
			t.details.Disks[name] = &DataTransferStat{
				Total:       total,
				Remaining:   remain,
				Transferred: sent,
				Progress:    int(sent * 100 / total),
				Speed:       int(((sent - t.details.Disks[name].Transferred) * 8) / 1 >> (10 * 2)), // mbit/s
			}
		}
	}

	percent := t.details.VMState.Progress

	for _, d := range t.details.Disks {
		percent += d.Progress
	}

	t.SetProgress(percent / (1 + len(t.details.Disks)))
}

func (t *MachineMigrationTask) Main() error {
	// The migration of virt.machine state and its disks
	// are running in the parallel goroutines.
	// These variables are to syncronize the states between goroutines.
	allDisksReady := make(chan struct{})
	vmstateMigrated := make(chan struct{})

	defer func() {
		// cleanup: close regardless of the Main() exit code
		select {
		case <-allDisksReady:
		default:
			close(allDisksReady)
		}
	}()

	group, ctx := errgroup.WithContext(t.Ctx())

	// Mirror the firmware flash device if exists
	if fwflash := t.vm.C.FirmwareGetFlash(); fwflash != nil {
		firmwareFlashReady := make(chan struct{})

		defer func() {
			// cleanup: close regardless of the Main() exit code
			select {
			case <-firmwareFlashReady:
			default:
				close(firmwareFlashReady)
			}
		}()

		group.Go(func() error {
			return t.mirrorDisks(ctx, []*kvmrun.Disk{fwflash}, firmwareFlashReady, vmstateMigrated)
		})

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-firmwareFlashReady:
			break
		}
	}

	// Start mirroring the specified disks
	if len(t.movingDisks) > 0 {
		group.Go(func() error {
			return t.mirrorDisks(ctx, t.movingDisks, allDisksReady, vmstateMigrated)
		})
	} else {
		close(allDisksReady)
	}

	// Start the migration of virt.machine state
	group.Go(func() error {
		return t.migrateMachineState(ctx, allDisksReady, vmstateMigrated)
	})

	// ... and wait for their completion
	return group.Wait()
}

func (t *MachineMigrationTask) OnSuccess() error {
	t.MachineCreateDownFile(t.vmname)

	// Forced shutdown if migration is successful
	if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "quit", Arguments: nil}, nil); err != nil {
		t.Logger.Warnf("Failed to terminate machine using command 'quit': %s", err)
	}

	testRemoteMachine := func() error {
		var resp *pb_machines.GetResponse

		err := t.KvmrunGRPC(t.dstServer, func(client *server.Kvmrun_Interfaces) (err error) {
			resp, err = client.Machines().Get(t.Ctx(), &pb_machines.GetRequest{Name: t.vmname})

			return err
		})
		if err != nil {
			return err
		}

		if t.requisites.PID != resp.Machine.PID {
			return fmt.Errorf("remote machine PID has changed since it was started: %d != %d", t.requisites.PID, resp.Machine.PID)
		}

		if t.turnOffAfter {
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

	if t.opts.RemoveAfter {
		t.Logger.Info("Local machine will be removed as requested (RemoveAfter == true)")

		err := func() error {
			// Make 3 attempts to make sure that remote machine works fine
			for attemp := 0; attemp < 3; attemp++ {
				time.Sleep(5 * time.Second)

				if err := testRemoteMachine(); err != nil {
					return err
				}
			}

			if err := t.SystemdStopService(t.vmname, 30*time.Second); err != nil {
				return fmt.Errorf("failed to shutdown %s: %s", t.vmname, err)
			}

			if err := t.SystemdDisableService(t.vmname); err != nil {
				return err
			}

			osuser.RemoveUser(t.vmname)

			if err := os.RemoveAll(filepath.Join(kvmrun.CONFDIR, t.vmname)); err != nil {
				return err
			}

			return nil
		}()
		if err != nil {
			t.Logger.Errorf("Failed to remove configuretion of %s: %s", t.vmname, err)
		}
	}

	return nil
}

func (t *MachineMigrationTask) OnFailure(taskErr error) {
	for _, d := range t.movingDisks {
		opts := struct {
			Device string `json:"device"`
			Force  bool   `json:"force,omitempty"`
		}{
			Device: "migr_" + d.BaseName(),
			Force:  true,
		}

		if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "block-job-cancel", Arguments: &opts}, nil); err != nil {
			// Non-fatal error. Just printing
			t.Logger.Warnf("OnFailureHook: forced block-job-cancel failed: %s", err)
		}
	}

	if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "migrate_cancel", Arguments: nil}, nil); err != nil {
		// Non-fatal error. Just printing
		t.Logger.Warnf("OnFailureHook: forced migrate_cancel failed: %s", err)
	}

	err := t.Server.KvmrunGRPC(t.dstServer, func(client *server.Kvmrun_Interfaces) (err error) {
		req := pb_tasks.CancelRequest{
			Key: t.vmname + "/incoming-migration",
		}

		_, err = client.Tasks().Cancel(t.Ctx(), &req)

		return err
	})
	if err == nil {
		t.Logger.Info("OnFailureHook: remote incoming task was successfully cancelled")
	} else {
		t.Logger.Warnf("OnFailureHook: failed to cancel the remote task: %s", err)
	}
}

func (t *MachineMigrationTask) migrateMachineState(ctx context.Context, allDisksReady, vmstateMigrated chan struct{}) error {
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

	var err error

	// Set capabilities
	err = func() error {
		t.Logger.Debug("migrateVMState(): run QMP command: migrate-set-capabilities")

		/*
			TODO: need to improve
				* first get a list,
				* then change only specified parameters
				* and then apply it uning migrate-set-capabilities
		*/

		args := struct {
			Capabilities []qemu_types.MigrationCapabilityStatus `json:"capabilities"`
		}{
			Capabilities: []qemu_types.MigrationCapabilityStatus{
				{Capability: "xbzrle", State: true},
				{Capability: "auto-converge", State: true},
				{Capability: "dirty-bitmaps", State: true},
			},
		}

		if t.vm.R.QemuVersion().Int() < 90000 { // < 9.x.x
			args.Capabilities = append(args.Capabilities, qemu_types.MigrationCapabilityStatus{Capability: "compress", State: false})
			args.Capabilities = append(args.Capabilities, qemu_types.MigrationCapabilityStatus{Capability: "block", State: false})
		}

		return t.Mon.Run(t.vmname, qmp.Command{Name: "migrate-set-capabilities", Arguments: &args}, nil)
	}()
	if err != nil {
		return err
	}

	// Parameters
	err = func() error {
		t.Logger.Debug("migrateVMState(): run QMP command: migrate-set-parameters")

		/*
			TODO: need to improve
				* first get a list,
				* then change only specified parameters
				* and then apply it uning migrate-set-parameters
		*/

		args := qemu_types.MigrateSetParameters{
			MaxBandwidth:    8589934592,
			XbzrleCacheSize: 536870912,
		}

		return t.Mon.Run(t.vmname, qmp.Command{Name: "migrate-set-parameters", Arguments: &args}, nil)
	}()
	if err != nil {
		return err
	}

	t.Logger.Debug("migrateVMState(): run QMP command: migrate; waiting for QEMU migration ...")

	// Start
	args := struct {
		URI string `json:"uri"`
	}{
		URI: fmt.Sprintf("tcp:%s:%d", t.dstServerAddr, t.requisites.IncomingPort),
	}

	if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "migrate", Arguments: &args}, nil); err != nil {
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

		if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "query-migrate", Arguments: nil}, mi); err != nil {
			return err
		}

		switch mi.Status {
		case "active", "postcopy-active", "completed":
			t.updateStat("vmstate", "", mi.Ram.Total, mi.Ram.Remaining, mi.Ram.Total-mi.Ram.Remaining, int(mi.Ram.Speed))
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

func (t *MachineMigrationTask) requestIncomingMigration() (*pb_types.IncomingMigrationRequisites, error) {
	diskSizes := make(map[string]uint64)

	// The keys in Overrides.Disks are already converted to full name
	for _, d := range t.movingDisks {
		if ovrd, ok := t.opts.Overrides.Disks[d.Path]; ok {
			diskSizes[ovrd] = d.QemuVirtualSize
		} else {
			diskSizes[d.Path] = d.QemuVirtualSize
		}
	}

	manifest, err := func() ([]byte, error) {
		var hasOverrides bool

		tmp := struct {
			Name string               `json:"name"`
			C    *kvmrun.InstanceConf `json:"conf"`
			R    *kvmrun.InstanceQemu `json:"run"`
		}{}

		if b, err := json.Marshal(t.vm); err == nil {
			if err := json.Unmarshal(b, &tmp); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}

		if len(t.opts.Overrides.Name) > 0 {
			tmp.Name = t.opts.Overrides.Name

			hasOverrides = true
		}

		if len(t.opts.Overrides.Disks) > 0 {
			for _, d := range tmp.C.Disks.Values() {
				if ovrd, ok := t.opts.Overrides.Disks[d.Path]; ok {
					d.Path = ovrd
				}
			}

			for _, d := range tmp.R.Disks.Values() {
				if ovrd, ok := t.opts.Overrides.Disks[d.Path]; ok {
					d.Path = ovrd
				}
			}

			hasOverrides = true
		}

		if len(t.opts.Overrides.NetIfaces) > 0 {
			for _, n := range tmp.C.NetIfaces.Values() {
				if ovrd, ok := t.opts.Overrides.NetIfaces[n.Ifname]; ok {
					n.Ifname = ovrd
				}
			}

			for _, n := range tmp.R.NetIfaces.Values() {
				if ovrd, ok := t.opts.Overrides.NetIfaces[n.Ifname]; ok {
					n.Ifname = ovrd
				}
			}

			hasOverrides = true
		}

		if hasOverrides {
			return json.Marshal(tmp)
		}

		return json.Marshal(t.vm)
	}()
	if err != nil {
		return nil, err
	}

	// Some extra files that may contain additional
	// configuration such as network settings, EFI boot, cloud-init etc.
	extraFiles, err := t.extraFiles()
	if err != nil {
		return nil, err
	}

	req := pb_system.StartIncomingMigrationRequest{
		Name:         t.vmname,
		Manifest:     manifest,
		ListenAddr:   t.dstServerAddr.String(),
		Disks:        diskSizes,
		CreateDisks:  t.opts.CreateDisks,
		ExtraFiles:   extraFiles,
		TurnOffAfter: t.turnOffAfter,
	}

	if len(t.opts.Overrides.Name) > 0 {
		req.Name = t.opts.Overrides.Name
	}

	var requisites *pb_types.IncomingMigrationRequisites

	err = t.Server.KvmrunGRPC(t.dstServer, func(client *server.Kvmrun_Interfaces) error {
		resp, err := client.System().StartIncomingMigration(t.Ctx(), &req)
		if err != nil {
			return err
		}

		requisites = resp.Requisites

		return err
	})

	if err != nil {
		return nil, err
	}

	return requisites, nil
}

// extraFiles returns a map including the contents of some extra files
// placed in the virtual machine directory.
// These files may contain additional configuration such as network settings
// (config_network).
func (t *MachineMigrationTask) extraFiles() (map[string][]byte, error) {
	r := regexp.MustCompile(`^(extra|comment|cloudinit_drive|ci_drive|config_[[:alnum:]]*|[\.\_[:alnum:]]*_config)$`)

	vmdir := filepath.Join(kvmrun.CONFDIR, t.vmname)

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

func (t *MachineMigrationTask) mirrorDisks(ctx context.Context, disks []*kvmrun.Disk, ready, stateMigrated chan struct{}) error {
	processor := machineMigrationTask_StorageMirroringProcessor{
		t:      t,
		ctx:    ctx,
		logger: t.Logger,
	}

	processor.disks = disks

	processor.readyToComplete = ready
	processor.machineStateMigrated = stateMigrated

	return processor.run()
}

//
// StorageMirroringProcessor
//

type machineMigrationTask_StorageMirroringProcessor struct {
	t *MachineMigrationTask

	ctx    context.Context
	logger *log.Entry

	disks                []*kvmrun.Disk
	readyToComplete      chan struct{}
	machineStateMigrated chan struct{}
}

func (p *machineMigrationTask_StorageMirroringProcessor) run() (err error) {
	diskNames := make([]string, 0, len(p.disks))

	for _, d := range p.disks {
		diskNames = append(diskNames, d.BaseName())
	}

	p.logger.Infof("Start disks mirroring process (%s)", strings.Join(diskNames, ", "))

	defer func() {
		if err == nil {
			p.logger.Infof("Disks mirroring process completed (%s)", strings.Join(diskNames, ", "))
		}
	}()

	group1, ctx1 := errgroup.WithContext(p.ctx)

	// Start the mirroring all disks.
	// For each disk, we run a separate goroutine,
	// where the Ready status is expected.
	for _, _d := range p.disks {
		d := _d

		p.logger.Infof("Run QMP command: drive-mirror; name=%s, remote_addr=%s:%d", d.BaseName(), p.t.dstServerAddr.String(), p.t.requisites.NBDPort)

		dstName := d.BaseName()

		if ovrd, ok := p.t.opts.Overrides.Disks[d.Path]; ok {
			if _d, err := kvmrun.NewDisk(ovrd); err == nil {
				dstName = _d.BaseName()
			} else {
				return err
			}
		}

		ts := time.Now()

		args := p.newMirrorOpts(d.BaseName(), dstName)

		if err := p.t.Server.Mon.Run(p.t.vmname, qmp.Command{Name: "drive-mirror", Arguments: args}, nil); err != nil {
			return fmt.Errorf("failed to start mirroring (%s): %w", d.BaseName(), err)
		}

		group1.Go(func() error { return p.waitForReady(ctx1, ts, d) })
	}

	p.t.Logger.Info("Wait for disks synchronization ...")

	if err := group1.Wait(); err != nil {
		return err
	}

	// All disks are ready. Notify dependent processes about this
	close(p.readyToComplete)

	p.logger.Info("All disks are synchronized")

	// Stat update and wait for machine state is migrated
	if err := p.waitMachineStateMigrated(); err != nil {
		return err
	}

	p.t.Logger.Info("Machine state migration is completed. Wait for total disks synchronization ...")

	group2, ctx2 := errgroup.WithContext(p.ctx)

	// Stop the mirroring
	for _, _d := range p.disks {
		d := _d

		p.logger.Infof("Run QMP command: block-job-complete; name=%s", d.BaseName())

		jobID := struct {
			Device string `json:"device"`
		}{
			Device: "migr_" + d.BaseName(),
		}

		ts := time.Now()

		if err := p.t.Server.Mon.Run(p.t.vmname, qmp.Command{Name: "block-job-complete", Arguments: &jobID}, nil); err != nil {
			// Non-fatal error, just printing
			p.t.Logger.Errorf("Failed to stop mirroring (%s): %s", d.BaseName(), err.Error())

			continue
		}

		group2.Go(func() error { return p.waitForCompleted(ctx2, ts, d) })
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

	return nil
}

func (p *machineMigrationTask_StorageMirroringProcessor) getJob(jobID string) (*qemu_types.BlockJobInfo, error) {
	jobs := make([]*qemu_types.BlockJobInfo, 0, len(p.disks))

	if err := p.t.Server.Mon.Run(p.t.vmname, qmp.Command{Name: "query-block-jobs", Arguments: nil}, &jobs); err != nil {
		return nil, err
	}

	for _, j := range jobs {
		if j.Device == jobID {
			return j, nil
		}
	}
	return nil, nil
}

func (p *machineMigrationTask_StorageMirroringProcessor) waitForReady(ctx context.Context, ts time.Time, d *kvmrun.Disk) error {
	jobID := "migr_" + d.BaseName()

	// Disk stat will be available after the job status changes to running
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := p.t.Server.Mon.WaitJobStatusChangeEvent(p.t.vmname, timeoutCtx, jobID, "running", uint64(ts.Unix())); err != nil {
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
			// No errors should be here
			return fmt.Errorf("failed to get mirroring status (jobID=%s): %w", jobID, err)
		}
		if job == nil {
			return fmt.Errorf("unable to get mirroring status: job ID = %s", jobID)
		}

		if d.BaseName() != "fwflash" {
			p.t.updateStat("disk", d.Path, d.QemuVirtualSize, job.Len-job.Offset, d.QemuVirtualSize-(job.Len-job.Offset), 0)
		}

		if job.Ready {
			break
		}
	}

	return nil
}

func (p *machineMigrationTask_StorageMirroringProcessor) waitMachineStateMigrated() error {
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
			jobID := "migr_" + d.BaseName()

			job, err := p.getJob(jobID)
			if err != nil {
				// No errors should be here
				return fmt.Errorf("failed to get mirroring status (jobID=%s): %w", jobID, err)
			}
			if job == nil {
				return fmt.Errorf("unable to get mirroring status: jobID = %s", jobID)
			}

			if d.BaseName() != "fwflash" {
				p.t.updateStat("disk", d.Path, d.QemuVirtualSize, job.Len-job.Offset, d.QemuVirtualSize-(job.Len-job.Offset), 0)
			}
		}
	}

	return nil
}

func (p *machineMigrationTask_StorageMirroringProcessor) waitForCompleted(ctx context.Context, ts time.Time, d *kvmrun.Disk) error {
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
		if err != nil {
			return fmt.Errorf("failed to complete disk mirroring for %s: %s", d.BaseName(), err)
		}

		// Ok, job completed
		if job == nil {
			if _, found, err := p.t.Server.Mon.FindBlockJobErrorEvent(p.t.vmname, jobID, uint64(ts.Unix())); err == nil {
				if found {
					return fmt.Errorf("errors detected during disk mirroring: %s", d.BaseName())
				}
			} else {
				return fmt.Errorf("FindBlockJobErrorEvent failed: %s: %w", d.BaseName(), err)
			}

			if _, found, err := p.t.Server.Mon.FindBlockJobCompletedEvent(p.t.vmname, jobID, uint64(ts.Unix())); err == nil {
				if !found {
					return fmt.Errorf("no completed event found: %s", d.BaseName())
				}
			} else {
				return fmt.Errorf("FindBlockJobCompletedEvent failed: %s: %w", d.BaseName(), err)
			}

			break
		}
	}

	return nil
}

func (p *machineMigrationTask_StorageMirroringProcessor) newMirrorOpts(srcName, dstName string) *qemu_types.DriveMirrorOptions {
	opts := qemu_types.DriveMirrorOptions{
		JobID:  fmt.Sprintf("migr_%s", srcName),
		Device: srcName,
		Target: fmt.Sprintf("nbd:%s:%d:exportname=%s", p.t.dstServerAddr.String(), int(p.t.requisites.NBDPort), dstName),
		Format: "nbd",
		Sync:   "full",
		Mode:   "existing",
	}

	if p.t.vm.R.QemuVersion().Int() >= 60000 { // >= 6.x.x
		opts.CopyMode = "write-blocking"
	}

	return &opts
}
