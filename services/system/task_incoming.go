package system

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/internal/lvm"
	"github.com/0xef53/kvmrun/internal/netstat"
	"github.com/0xef53/kvmrun/internal/osuser"
	"github.com/0xef53/kvmrun/internal/qemu"
	qemu_types "github.com/0xef53/kvmrun/internal/qemu/types"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/services"

	pb "github.com/0xef53/kvmrun/api/services/system/v1"
	pb_types "github.com/0xef53/kvmrun/api/types"

	qmp "github.com/0xef53/go-qmp/v2"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

type IncomingMachineTask struct {
	*task.GenericTask
	*services.ServiceServer

	req *pb.StartIncomingMachineRequest

	requisites       *pb_types.IncomingMachineRequisites
	listenAddr       net.IP
	hasFirmwareFlash bool
	createdDisks     []string
}

func NewIncomingMachineTask(req *pb.StartIncomingMachineRequest, ss *services.ServiceServer) *IncomingMachineTask {
	return &IncomingMachineTask{
		GenericTask:   new(task.GenericTask),
		ServiceServer: ss,
		req:           req,
	}
}

func (t *IncomingMachineTask) GetNS() string { return "machine-incoming" }

func (t *IncomingMachineTask) GetKey() string { return t.req.Name + "::" }

func (t *IncomingMachineTask) BeforeStart(resp interface{}) error {
	t.requisites = new(pb_types.IncomingMachineRequisites)

	if v, ok := resp.(*pb.StartIncomingMachineResponse); ok {
		v.Requisites = t.requisites
	} else {
		panic(fmt.Sprintf("incorrect type of resp interface: %T", resp))
	}

	// A quick check if a machine with this name exists.
	if _, err := os.Stat(filepath.Join(kvmrun.CONFDIR, t.req.Name, "config")); err == nil {
		return grpc_status.Errorf(grpc_codes.AlreadyExists, "already exists: %s", t.req.Name)
	}

	var success bool

	defer func() {
		if !success {
			t.OnFailure()
		}
	}()

	if t.req.CreateDisks {
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

	vmconf, err := kvmrun.GetInstanceConf(t.req.Name)
	if err != nil {
		return err
	}

	if vmconf.GetFirmwareFlash() != nil {
		t.hasFirmwareFlash = true
	}

	if err := t.startNBDServer(kvmrun.FIRST_NBD_PORT + vmconf.Uid()); err != nil {
		return err
	}

	/*
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

	t.requisites.IncomingPort = int32(kvmrun.FIRST_INCOMING_PORT + vmconf.Uid())
	t.requisites.NBDPort = int32(kvmrun.FIRST_NBD_PORT + vmconf.Uid())
	t.requisites.Pid = pid

	success = true

	return nil
}

func (t *IncomingMachineTask) Main() error {
	done := make(chan struct{})
	defer close(done)

	// This function is used to stop the task if there are no connections
	// on the data ports for 60 seconds.
	// For example, something happened on the source host-server
	// and it could not notify this task about the problems.
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		var failcnt int

		for {
			select {
			case <-done:
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
				t.Logger.Warnf("Watcher: failed to check established connections: %s", err)
			}
		}

		t.Logger.Info("Watcher: no one established connection found in the last 60 seconds. Canceling the task")

		t.Cancel()
	}()

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

		if err := t.Mon.Run(t.req.Name, qmp.Command{"query-status", nil}, &st); err != nil {
			return err
		}

		switch st.Status {
		case "inmigrate", "finish-migrate", "postmigrate":
			continue
		}

		t.Logger.Infof("Got a new QEMU status: %s. Data transfer has completed successfully", st.Status)

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
			return fmt.Errorf("Failed to check established connections: %s", err)
		}

		if len(conns) > 0 {
			continue
		}

		t.Logger.Info("All incoming connections are completed")

		break
	}

	t.Logger.Info("Wait for the migration to complete")

	mi := qemu_types.MigrationInfo{}

	if err := t.Mon.Run(t.req.Name, qmp.Command{"query-migrate", nil}, &mi); err == nil {
		if mi.Status != "completed" {
			return err
		}
	} else {
		t.Logger.Warnf("Failed to request the migration status: %s", err)
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

func (t *IncomingMachineTask) OnSuccess() error {
	if t.req.TurnOffAfter {
		t.Logger.Info("The machine will be turned off as requested (TurnOffAfter == true)")

		ioutil.WriteFile(t.MachineDownFile(t.req.Name), []byte(""), 0644)

		if err := t.SystemCtl.StopAndWait(t.MachineToUnit(t.req.Name), 60*time.Second, nil); err != nil {
			t.Logger.Errorf("Unable to turn off the machine: %s", err)
		}
	} else {
		if err := t.Mon.Run(t.req.Name, qmp.Command{"cont", nil}, nil); err != nil {
			t.Logger.Errorf("Failed to send CONT signal via QMP: %s", err)
		}
	}

	return nil
}

func (t *IncomingMachineTask) OnFailure() error {
	t.SystemCtl.KillBySIGTERM(t.MachineToUnit(t.req.Name))

	if err := t.SystemCtl.StopAndWait(t.MachineToUnit(t.req.Name), 30*time.Second, nil); err != nil {
		t.Logger.Warnf("OnFailureHook: failed to gracefully stop the incoming machine: %s", err)
	}

	if err := t.SystemCtl.Disable(t.MachineToUnit(t.req.Name)); err != nil {
		t.Logger.Warnf("OnFailureHook: failed to disable the systemd unit: %s", err)
	}

	osuser.RemoveUser(t.req.Name)

	dirs := []string{
		filepath.Join(kvmrun.CONFDIR, t.req.Name),
		filepath.Join(kvmrun.LOGDIR, t.req.Name),
	}

	for _, d := range dirs {
		if err := os.RemoveAll(d); err != nil {
			t.Logger.Warnf("OnFailureHook: %s", err)
		}
	}

	t.Logger.Infof("OnFailureHook: removed: %s", t.req.Name)

	for _, diskPath := range t.createdDisks {
		if err := lvm.RemoveVolume(diskPath); err != nil {
			t.Logger.Warnf("OnFailureHook: %s", err)
		}
		t.Logger.Infof("OnFailureHook: removed: %s", diskPath)
	}

	return nil
}

func (t *IncomingMachineTask) getEstablishedConnections() ([]netstat.SockTableEntry, error) {
	listenAddr := net.ParseIP(t.req.ListenAddr)

	filterFn := func(s *netstat.SockTableEntry) bool {
		if s.State == netstat.Established {
			if s.LocalAddr.IP.Equal(listenAddr) || s.LocalAddr.IP.Equal(net.IPv4(0, 0, 0, 0)) {
				switch int32(s.LocalAddr.Port) {
				case t.requisites.IncomingPort, t.requisites.NBDPort:
					return true
				}
			}
		}
		return false
	}

	return netstat.TCPSocks(filterFn)
}

func (t *IncomingMachineTask) validateDisks() error {
	for diskPath, diskSize := range t.req.Disks {
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

func (t *IncomingMachineTask) createLocalDisks() error {
	for diskPath, diskSize := range t.req.Disks {
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

func (t *IncomingMachineTask) startIncomingMachine() (int32, error) {
	var incvm incomingMachine

	if err := json.Unmarshal(t.req.Manifest, &incvm); err != nil {
		return 0, err
	}

	vmdir := filepath.Join(kvmrun.CONFDIR, t.req.Name)
	vmlogdir := filepath.Join(kvmrun.LOGDIR, t.req.Name)

	// A quick check if a machine with this name exists.
	if _, err := os.Stat(filepath.Join(vmdir, "config")); err == nil {
		return 0, grpc_status.Errorf(grpc_codes.AlreadyExists, "already exists: %s", t.req.Name)
	}

	for _, d := range []string{vmdir, vmlogdir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return 0, err
		}
	}

	if _, err := osuser.CreateUser(t.req.Name); err != nil {
		return 0, err
	}

	// Write the config file
	if err := incvm.C.Save(); err != nil {
		return 0, err
	}

	// Extra files
	if t.req.ExtraFiles != nil {
		for fname, content := range t.req.ExtraFiles {
			if err := ioutil.WriteFile(filepath.Join(kvmrun.CONFDIR, t.req.Name, fname), content, 0644); err != nil {
				return 0, err
			}
		}
	}

	mtype, err := qemu.DefaultMachineType()
	if err != nil {
		return 0, err
	}
	if incvm.R.GetMachineType().String() == mtype {
		incvm.R.SetMachineType("")
	}

	// Write the incoming_config file
	if err := incvm.R.Save(); err != nil {
		return 0, err
	}

	// Enable, start and test
	if err := t.SystemCtl.Enable(t.MachineToUnit(t.req.Name)); err != nil {
		return 0, err
	}

	if err := t.SystemCtl.StartAndTest(t.MachineToUnit(t.req.Name), 10*time.Second, nil); err != nil {
		return 0, err
	}

	return func() (int32, error) {
		b, err := ioutil.ReadFile(filepath.Join(kvmrun.CHROOTDIR, t.req.Name, "pid"))
		if err != nil {
			return 0, err
		}

		pid, err := strconv.Atoi(string(b))
		if err != nil {
			return 0, err
		}

		return int32(pid), nil
	}()
}

func (t *IncomingMachineTask) startNBDServer(port int) error {
	opts := struct {
		Addr qemu_types.InetSocketAddressLegacy `json:"addr"`
	}{
		Addr: qemu_types.InetSocketAddressLegacy{
			Type: "inet",
			Data: qemu_types.InetSocketAddressBase{
				Host: t.req.ListenAddr,
				Port: strconv.Itoa(port),
			},
		},
	}

	if err := t.Mon.Run(t.req.Name, qmp.Command{"nbd-server-start", &opts}, nil); err != nil {
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
		if err := t.Mon.Run(t.req.Name, qmp.Command{"nbd-server-add", &opts}, nil); err != nil {
			return fmt.Errorf("failed to export (fwflash): %w", err)
		}
	}

	for diskPath := range t.req.Disks {
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
		if err := t.Mon.Run(t.req.Name, qmp.Command{"nbd-server-add", &opts}, nil); err != nil {
			return fmt.Errorf("failed to export (%s): %w", d.BaseName(), err)
		}
	}

	return nil
}

func (t *IncomingMachineTask) stopNBDServer() error {
	return t.Mon.Run(t.req.Name, qmp.Command{"nbd-server-stop", nil}, nil)
}
