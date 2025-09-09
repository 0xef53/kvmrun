package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/internal/dotenv"
	"github.com/0xef53/kvmrun/internal/lvm"
	"github.com/0xef53/kvmrun/internal/netstat"
	"github.com/0xef53/kvmrun/internal/osuser"
	"github.com/0xef53/kvmrun/internal/qemu"
	qemu_types "github.com/0xef53/kvmrun/internal/qemu/types"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/internal/version"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	qmp "github.com/0xef53/go-qmp/v2"
)

type IncomingMigrationOptions struct {
	Manifest     []byte            `json:"manifest"`
	Disks        map[string]uint64 `json:"disks"`
	ExtraFiles   map[string][]byte `json:"extra_files"`
	ListenAddr   string            `json:"listen_addr"`
	CreateDisks  bool              `json:"create_disks"`
	TurnOffAfter bool              `json:"turn_off_after"`
}

func (o *IncomingMigrationOptions) Validate(strict bool) error {
	if len(o.Manifest) == 0 {
		return fmt.Errorf("empty machine manifest")
	}

	if strict {
		var v interface{}

		if err := json.Unmarshal(o.Manifest, &v); err != nil {
			return fmt.Errorf("invalid manifest struct: %w", err)
		}
	}

	for diskname, size := range o.Disks {
		if len(strings.TrimSpace(diskname)) == 0 {
			return fmt.Errorf("empty diskname in 'disks' map")
		}

		if size == 0 {
			return fmt.Errorf("invalid size of disk '%s': got 0b, but must be greater than 0", diskname)
		}
	}

	return nil
}

type IncomingRequisites struct {
	IncomingPort int `json:"incoming_port"`
	NBDPort      int `json:"nbd_port"`
	PID          int `json:"pid"`
}

func (s *Server) StartIncomingMigrationProcess(ctx context.Context, vmname string, opts *IncomingMigrationOptions) (*IncomingRequisites, error) {
	if opts == nil {
		return nil, fmt.Errorf("empty incoming-migration opts")
	}

	t := NewMachineIncomingMigrationTask(vmname, opts)

	t.Server = s

	taskOpts := []task.TaskOption{
		server.WithUniqueLabel(vmname + "/incoming-migration"),
		server.WithGroupLabel(vmname),
		server.WithGroupLabel(vmname + "/long-running"),
	}

	requisites := IncomingRequisites{}

	_, err := s.TaskStart(ctx, t, &requisites, taskOpts...)
	if err != nil {
		return nil, fmt.Errorf("cannot start incoming instance: %w", err)
	}

	return &requisites, nil
}

type MachineIncomingMigrationTask struct {
	*task.GenericTask
	*Server

	targets map[string]task.OperationMode

	// Arguments
	vmname string
	opts   *IncomingMigrationOptions

	// Do not set manually next fields !
	requisites       *IncomingRequisites
	hasFirmwareFlash bool
	createdDisks     []string
}

func NewMachineIncomingMigrationTask(vmname string, opts *IncomingMigrationOptions) *MachineIncomingMigrationTask {
	return &MachineIncomingMigrationTask{
		GenericTask: new(task.GenericTask),
		targets:     server.BlockAnyOperations(vmname),
		vmname:      vmname,
		opts:        opts,
	}
}

func (t *MachineIncomingMigrationTask) Targets() map[string]task.OperationMode { return t.targets }

func (t *MachineIncomingMigrationTask) BeforeStart(resp interface{}) (err error) {
	if _, ok := resp.(*IncomingRequisites); !ok {
		return fmt.Errorf("unexpected error: invalid type of resp interface")
	}

	if t.opts == nil {
		return fmt.Errorf("empty incoming-migration opts")
	} else {
		if err := t.opts.Validate(true); err != nil {
			return err
		}
	}

	// Check if a machine with this name exists
	switch _, err := t.Server.MachineGet(t.vmname, false); {
	case err == nil:
		return fmt.Errorf("%w: %s", kvmrun.ErrAlreadyExists, t.vmname)
	case errors.Is(err, kvmrun.ErrNotFound):
		// OK
	default:
		return err
	}

	// Cleanup
	defer func() {
		if err != nil {
			t.OnFailure(err)
		}
	}()

	if t.opts.CreateDisks {
		if err := t.createLocalDisks(); err != nil {
			return err
		}
	} else {
		if err := t.validateDisks(); err != nil {
			return err
		}
	}

	pid, err := t.startIncomingMachine()
	if err != nil {
		return err
	}

	vmconf, err := kvmrun.GetInstanceConf(t.vmname)
	if err != nil {
		return err
	}

	if fwflash := vmconf.FirmwareGetFlash(); fwflash != nil {
		t.hasFirmwareFlash = true
	}

	if err := t.startNBDServer(kvmrun.FIRST_NBD_PORT + vmconf.UID()); err != nil {
		return err
	}

	/*
		TODO: need to set exactly the same ones as on the SRC server

		t.Logger.Debug("Set migration capabilities")

		capsArgs := struct {
			Capabilities []qemu_types.MigrationCapabilityStatus `json:"capabilities"`
		}{
			Capabilities: []qemu_types.MigrationCapabilityStatus{
				{"xbzrle", true},
				{"auto-converge", true},
				{"compress", false},
				{"block", false},
				{"dirty-bitmaps", true},
				{"late-block-activate", true},
			},
		}
		if err := t.Mon.Run(t.req.Name, qmp.Command{"migrate-set-capabilities", &capsArgs}, nil); err != nil {
			return err
		}
	*/

	t.requisites = &IncomingRequisites{
		IncomingPort: kvmrun.FIRST_INCOMING_PORT + vmconf.UID(),
		NBDPort:      kvmrun.FIRST_NBD_PORT + vmconf.UID(),
		PID:          int(pid),
	}

	// Return copy of requisites
	if v, ok := resp.(*IncomingRequisites); ok {
		v.IncomingPort = t.requisites.IncomingPort
		v.NBDPort = t.requisites.NBDPort
		v.PID = t.requisites.PID
	}

	return nil
}

func (t *MachineIncomingMigrationTask) Main() error {
	done := make(chan struct{})
	defer close(done)

	// Start monitoring incoming connections
	go t.monitorIncomingConnection(done)

	// Monitoring the migration process
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	st := struct {
		Status string `json:"status"`
	}{}

	t.Logger.Info("Wait for a data transfer to complete")

	for {
		select {
		case <-t.Ctx().Done():
			return t.Ctx().Err()
		case <-ticker.C:
		}

		if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "query-status", Arguments: nil}, &st); err != nil {
			return err
		}

		switch st.Status {
		case "inmigrate", "finish-migrate", "postmigrate":
			continue
		}

		t.Logger.Infof("Data transfer completed successfully: QEMU status = %s", st.Status)

		break
	}

	t.Logger.Info("Wait for all incoming connections to complete")

	for {
		select {
		case <-t.Ctx().Done():
			return t.Ctx().Err()
		case <-ticker.C:
		}

		conns, err := t.getEstablishedConnections()
		if err != nil {
			return fmt.Errorf("failed to check established connections: %w", err)
		}

		if len(conns) > 0 {
			continue
		}

		t.Logger.Info("All incoming connections are completed")

		break
	}

	t.Logger.Info("Wait for machine state migration to complete")

	mi := qemu_types.MigrationInfo{}

	if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "query-migrate", Arguments: nil}, &mi); err == nil {
		if mi.Status == "completed" {
			t.Logger.Info("Machine state migration completed")
		} else {
			return err
		}
	} else {
		t.Logger.Warnf("Failed to request migration status: %s", err)
	}

	if err := t.stopNBDServer(); err != nil {
		t.Logger.Warnf("Failed to stop NBD server: %s", err)
	}

	// Last chance to be interrupted by the source host-server
	select {
	case <-t.Ctx().Done():
		return t.Ctx().Err()
	case <-ticker.C:
	}

	return nil
}

func (t *MachineIncomingMigrationTask) OnSuccess() error {
	if t.opts.TurnOffAfter {
		t.Logger.Info("Machine will be turned off as requested (TurnOffAfter == true)")

		t.MachineCreateDownFile(t.vmname)

		if err := t.SystemdStopService(t.vmname, 60*time.Second); err != nil {
			t.Logger.Errorf("Unable to turn off: %s", err)
		}
	} else {
		if err := t.Server.Mon.Run(t.vmname, qmp.Command{Name: "cont", Arguments: nil}, nil); err != nil {
			t.Logger.Errorf("Failed to send CONT signal via QMP: %s", err)
		}
	}

	return nil
}

func (t *MachineIncomingMigrationTask) OnFailure(taskErr error) {
	t.SystemdSendSIGTERM(t.vmname)

	if err := t.SystemdStopService(t.vmname, 30*time.Second); err != nil {
		t.Logger.Warnf("OnFailureHook: failed to gracefully stop the incoming machine: %s", err)
	}

	if err := t.SystemdDisableService(t.vmname); err != nil {
		t.Logger.Warnf("OnFailureHook: failed to disable the systemd unit: %s", err)
	}

	osuser.RemoveUser(t.vmname)

	if err := os.RemoveAll(filepath.Join(kvmrun.CONFDIR, t.vmname)); err != nil {
		t.Logger.Warnf("OnFailureHook: %s", err)
	}

	t.Logger.Infof("OnFailureHook: removed: %s", t.vmname)

	for _, diskPath := range t.createdDisks {
		if err := lvm.RemoveVolume(diskPath); err != nil {
			t.Logger.Warnf("OnFailureHook: %s", err)
		}
		t.Logger.Infof("OnFailureHook: removed: %s", diskPath)
	}
}

// monitorIncomingConnection stops the task if there are no connections
// on the data ports in the last 60 seconds.
//
// For example, something happened on the source host-server
// and it could not notify this task about the problems.
func (t *MachineIncomingMigrationTask) monitorIncomingConnection(taskCompleted chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var failcnt int

	for {
		select {
		case <-taskCompleted:
			return
		case <-ticker.C:
		}

		if failcnt == 12 {
			break
		}

		if conns, err := t.getEstablishedConnections(); err == nil {
			if len(conns) > 0 {
				failcnt = 0
			} else {
				failcnt++
			}
		} else {
			t.Logger.Warnf("ConnMonitor: failed to check established connections: %s", err)
		}
	}

	t.Logger.Info("ConnMonitor: no one established connection found in the last 60 seconds. Canceling the task")

	t.Cancel()
}

func (t *MachineIncomingMigrationTask) getEstablishedConnections() ([]netstat.SockTableEntry, error) {
	listenAddr := net.ParseIP(t.opts.ListenAddr)

	if listenAddr == nil {
		return nil, fmt.Errorf("cannot parse listen addr: %s", t.opts.ListenAddr)
	}

	filterFn := func(s *netstat.SockTableEntry) bool {
		if s.State == netstat.Established {
			if s.LocalAddr.IP.Equal(listenAddr) || s.LocalAddr.IP.Equal(net.IPv4(0, 0, 0, 0)) {
				switch int(s.LocalAddr.Port) {
				case t.requisites.IncomingPort, t.requisites.NBDPort:
					return true
				}
			}
		}

		return false
	}

	return netstat.TCPSocks(filterFn)
}

func (t *MachineIncomingMigrationTask) validateDisks() error {
	for diskPath, diskSize := range t.opts.Disks {
		d, err := kvmrun.NewDisk(diskPath)
		if err != nil {
			return err
		}

		if ok, err := d.IsAvailable(); !ok {
			return err
		}

		if d.IsLocal() {
			size, err := d.Backend.Size()
			if err != nil {
				return err
			}

			if size < diskSize {
				return fmt.Errorf("insufficient space on %s", diskPath)
			}
		}
	}

	return nil
}

func (t *MachineIncomingMigrationTask) createLocalDisks() error {
	for diskPath, diskSize := range t.opts.Disks {
		d, err := kvmrun.NewDisk(diskPath)
		if err != nil {
			return err
		}

		switch ok, err := d.IsAvailable(); {
		case err == nil:
			if ok {
				return fmt.Errorf("disk already exists: %s", diskPath)
			}
		case os.IsNotExist(err):
		default:
			return err
		}

		if d.IsLocal() {
			ff := strings.Split(diskPath[1:], "/")
			if len(ff) != 3 {
				return fmt.Errorf("no idea how to create a block device: %s", diskPath)
			}

			if err := lvm.CreateVolume(ff[1], ff[2], diskSize); err != nil {
				return err
			}

			t.createdDisks = append(t.createdDisks, diskPath)
		}
	}

	return nil
}

func (t *MachineIncomingMigrationTask) startNBDServer(port int) error {
	opts := struct {
		Addr qemu_types.InetSocketAddressLegacy `json:"addr"`
	}{
		Addr: qemu_types.InetSocketAddressLegacy{
			Type: "inet",
			Data: qemu_types.InetSocketAddressBase{
				Host: t.opts.ListenAddr,
				Port: strconv.Itoa(port),
			},
		},
	}

	if err := t.Mon.Run(t.vmname, qmp.Command{Name: "nbd-server-start", Arguments: &opts}, nil); err != nil {
		return err
	}

	if t.hasFirmwareFlash {
		opts := struct {
			Device   string `json:"device"`
			Writable bool   `json:"writable"`
		}{
			Device:   "fwflash",
			Writable: true,
		}
		if err := t.Mon.Run(t.vmname, qmp.Command{Name: "nbd-server-add", Arguments: &opts}, nil); err != nil {
			return fmt.Errorf("failed to export (fwflash): %w", err)
		}
	}

	for diskPath := range t.opts.Disks {
		d, err := kvmrun.NewDisk(diskPath)
		if err != nil {
			return err
		}
		opts := struct {
			Device   string `json:"device"`
			Writable bool   `json:"writable"`
		}{
			Device:   d.BaseName(),
			Writable: true,
		}
		if err := t.Mon.Run(t.vmname, qmp.Command{Name: "nbd-server-add", Arguments: &opts}, nil); err != nil {
			return fmt.Errorf("failed to export (%s): %w", d.BaseName(), err)
		}
	}

	return nil
}

func (t *MachineIncomingMigrationTask) stopNBDServer() error {
	return t.Mon.Run(t.vmname, qmp.Command{Name: "nbd-server-stop", Arguments: nil}, nil)
}

// incomingMachine is an auxiliary structure to correctly converting
// incoming JSON to kvmrun.VirtMachine.
type incomingMachine struct {
	kvmrun.Machine
}

func (m *incomingMachine) UnmarshalJSON(data []byte) error {
	tmp := struct {
		Name string          `json:"name"`
		R    json.RawMessage `json:"run"`
		C    json.RawMessage `json:"conf"`
	}{}

	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	vmc := kvmrun.NewInstanceConf(tmp.Name)
	vmr := kvmrun.NewIncomingConf(tmp.Name)

	if err := json.Unmarshal(tmp.C, &vmc); err != nil {
		return err
	}

	if len(tmp.R) != 0 {
		if err := json.Unmarshal(tmp.R, &vmr); err != nil {
			return err
		}
	} else {
		vmr = nil
	}

	m.Name = tmp.Name
	m.C = vmc
	m.R = vmr

	return nil
}

func (t *MachineIncomingMigrationTask) startIncomingMachine() (uint32, error) {
	var incvm incomingMachine

	if err := json.Unmarshal(t.opts.Manifest, &incvm); err != nil {
		return 0, err
	}

	vmdir := filepath.Join(kvmrun.CONFDIR, t.vmname)

	if err := os.MkdirAll(vmdir, 0755); err != nil {
		return 0, err
	}

	// Write the config file
	if err := incvm.C.Save(); err != nil {
		return 0, err
	}

	// Extra files
	for fname, content := range t.opts.ExtraFiles {
		if err := os.WriteFile(filepath.Join(vmdir, fname), content, 0644); err != nil {
			return 0, err
		}
	}

	// OS user
	if _, err := osuser.CreateUser(t.vmname); err != nil {
		return 0, err
	}

	// Check QEMU that will run this machine
	if qver, err := t.getQemuVersion(); err == nil {
		if err := qemu.VerifyVersion(qver.String()); err == nil {
			if qemu.IsDefaultMachineType(qver.String(), incvm.R.MachineTypeGet().String()) {
				t.Logger.Infof("Machine type on source and destination servers are the same")

				// It's not a mistake, we are actually using "R" here
				incvm.R.MachineTypeSet("")
			}
		} else {
			return 0, err
		}
	} else {
		return 0, err
	}

	// Write the incoming_config file
	if err := incvm.R.Save(); err != nil {
		return 0, err
	}

	// Enable, start and test
	if err := t.SystemdEnableService(t.vmname); err != nil {
		return 0, err
	}

	if err := t.SystemdStartService(t.vmname, 10*time.Second); err != nil {
		return 0, err
	}

	// Return PID
	return func() (uint32, error) {
		b, err := os.ReadFile(filepath.Join(kvmrun.CHROOTDIR, t.vmname, "pid"))
		if err != nil {
			return 0, err
		}

		pid, err := strconv.Atoi(string(b))
		if err != nil {
			return 0, err
		}

		return uint32(pid), nil
	}()
}

func (t *MachineIncomingMigrationTask) getQemuVersion() (*version.Version, error) {
	qemuRootDir := t.AppConf.Kvmrun.QemuRootDir

	if vmenvs, err := dotenv.Read(filepath.Join(filepath.Join(kvmrun.CONFDIR, t.vmname), "config_envs")); err == nil {
		if dir, ok := vmenvs["QEMU_ROOTDIR"]; ok {
			if v, err := filepath.Abs(dir); err == nil {
				qemuRootDir = v
			} else {
				return nil, err
			}
		}
	} else {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if _, err := os.Stat(qemuRootDir); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("QEMU root directory does not exist: %s", qemuRootDir)
		}
		return nil, fmt.Errorf("failed to check QEMU root directory: %w", err)
	}

	t.Logger.Infof("QEMU root directory: %s", qemuRootDir)

	return qemu.GetVersion(qemuRootDir, kvmrun.QEMU_BINARY)
}
