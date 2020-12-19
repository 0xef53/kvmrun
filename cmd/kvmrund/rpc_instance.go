package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	"github.com/0xef53/kvmrun/pkg/ps"
	"github.com/0xef53/kvmrun/pkg/pwd"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"

	qmp "github.com/0xef53/go-qmp/v2"
	log "github.com/sirupsen/logrus"
)

func (h *rpcHandler) CreateConfInstance(r *http.Request, args *rpccommon.NewInstanceRequest, resp *struct{}) error {
	vmdir := filepath.Join(kvmrun.CONFDIR, args.Name)
	vmlogdir := filepath.Join(kvmrun.LOGDIR, args.Name)

	if _, err := os.Stat(filepath.Join(vmdir, "config")); err == nil {
		return fmt.Errorf("Already exists: %s", vmdir)
	}

	for _, d := range []string{vmdir, vmlogdir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	if _, err := pwd.CreateUser(args.Name); err != nil {
		return err
	}

	vmc := kvmrun.NewInstanceConf(args.Name)

	vmc.SetTotalMem(args.MemTotal)
	vmc.SetActualMem(args.MemActual)
	vmc.SetTotalCPUs(args.CPUTotal)
	vmc.SetActualCPUs(args.CPUActual)
	vmc.SetCPUQuota(args.CPUQuota)
	vmc.SetCPUModel(args.CPUModel)

	if err := vmc.Save(); err != nil {
		return err
	}

	return h.sctl.Enable(args.Name)
}

func (h *rpcHandler) CreateConfInstanceFromManifest(r *http.Request, args *rpccommon.NewManifestInstanceRequest, port *int) error {
	var vm IncomingVM

	if err := json.Unmarshal(args.Manifest, &vm); err != nil {
		return err
	}

	vmdir := filepath.Join(kvmrun.CONFDIR, vm.Name)
	vmlogdir := filepath.Join(kvmrun.LOGDIR, vm.Name)

	if _, err := os.Stat(filepath.Join(vmdir, "config")); err == nil {
		return fmt.Errorf("Already exists: %s", vmdir)
	}

	for _, d := range []string{vmdir, vmlogdir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	if err := vm.C.Save(); err != nil {
		return err
	}

	// Extra files
	if args.ExtraFiles != nil {
		for fname, content := range args.ExtraFiles {
			if err := ioutil.WriteFile(filepath.Join(kvmrun.CONFDIR, vm.Name, fname), content, 0644); err != nil {
				return err
			}
		}
	}

	if _, err := pwd.CreateUser(vm.Name); err != nil {
		return err
	}

	return h.sctl.Enable(vm.Name)
}

func (h *rpcHandler) RemoveConfInstance(r *http.Request, args *rpccommon.InstanceRequest, port *int) error {
	h.sctl.KillBySIGKILL(args.Name)

	if err := h.sctl.StopAndWait(args.Name, 30*time.Second, nil); err != nil {
		log.Errorf("Failed to shutdown %s: %s", args.Name, err)
	}

	if err := h.sctl.Disable(args.Name); err != nil {
		return err
	}

	pwd.RemoveUser(args.Name)

	dirs := []string{
		filepath.Join(kvmrun.CONFDIR, args.Name),
		filepath.Join(kvmrun.LOGDIR, args.Name),
	}
	for _, d := range dirs {
		if err := os.RemoveAll(d); err != nil {
			return err
		}
	}

	return nil
}

func (h *rpcHandler) GetInstanceJSON(r *http.Request, args *rpccommon.InstanceRequest, resp *[]byte) error {
	b, err := json.MarshalIndent(args.VM, "", "    ")
	if err != nil {
		return err
	}
	*resp = b

	return nil
}

func (h *rpcHandler) IsInstanceRunning(r *http.Request, args *rpccommon.InstanceRequest, resp *bool) error {
	if args.VM.R != nil {
		*resp = true
	}

	return nil
}

func (h *rpcHandler) IsConfExist(r *http.Request, args *rpccommon.VMNameRequest, resp *bool) error {
	switch _, err := os.Stat(filepath.Join(kvmrun.CONFDIR, args.Name, "config")); {
	case err == nil:
		*resp = true
	case os.IsNotExist(err):
	default:
		return err
	}

	return nil
}

func (h *rpcHandler) GetInstanceNames(r *http.Request, args *struct{}, resp *[]string) error {
	nn, err := getVMNames()
	if err != nil {
		return err
	}

	*resp = nn

	return nil
}

func (h *rpcHandler) StartSystemdUnit(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	return h.sctl.Start(args.Name, nil)
}

func (h *rpcHandler) StopSystemdUnit(r *http.Request, args *rpccommon.VMShutdownRequest, resp *struct{}) error {
	if args.Wait {
		return h.sctl.StopAndWait(args.Name, 60*time.Second, nil)
	}

	return h.sctl.Stop(args.Name, nil)
}

func (h *rpcHandler) RestartSystemdUnit(r *http.Request, args *rpccommon.VMShutdownRequest, resp *struct{}) error {
	if _, err := h.sctl.GetUnit(args.Name); err != nil {
		return nil
	}

	return h.sctl.Restart(args.Name, nil)
}

func (h *rpcHandler) ResetSystemdUnit(r *http.Request, args *rpccommon.VMShutdownRequest, resp *struct{}) error {
	if _, err := h.sctl.GetUnit(args.Name); err != nil {
		return nil
	}

	h.sctl.KillBySIGKILL(args.Name)

	if err := h.sctl.StopAndWait(args.Name, 30*time.Second, nil); err != nil {
		log.Errorf("Failed to shutdown %s: %s", args.Name, err)
	}

	return h.sctl.Start(args.Name, nil)
}

func (h *rpcHandler) StopQemuInstance(r *http.Request, args *rpccommon.VMShutdownRequest, resp *struct{}) error {
	run := func(c string) error {
		switch err := h.mon.Run(args.Name, qmp.Command{c, nil}, nil); err.(type) {
		case nil:
		case *net.OpError:
			// It means the socket is closed, i.e. virt.machine is not running
			return &kvmrun.NotRunningError{args.Name}
		default:
			return err
		}

		return nil
	}

	log.WithField("vmname", args.Name).Info("Forced resuming the emulation")

	if err := run("cont"); err != nil {
		return err
	}

	terminated := make(chan struct{})
	timeout := make(chan struct{})

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		// Repeat SYSTEM_POWERDOWN every second
		// just to be sure that the guest will receive it
		for {
			if err := run("system_powerdown"); err != nil {
				// It means the socket is closed. That is what we need
				break
			}
			select {
			case <-timeout:
				return
			case <-ticker.C:
			}
		}
		close(terminated)
	}()

	if args.Timeout < 1*time.Second {
		args.Timeout = 1 * time.Second
	}

	wait := func() error {
		select {
		case <-time.After(args.Timeout):
			close(timeout)
			log.WithField("vmname", args.Name).Warn("Timed out: sending quit signal")
			if err := run("quit"); err != nil && !kvmrun.IsNotRunningError(err) {
				return err
			}
		case <-terminated:
			log.WithField("vmname", args.Name).Info("Has been terminated")
		}
		return nil
	}

	if args.Wait {
		return wait()
	}

	go wait()

	return nil
}

func (h *rpcHandler) GetQemuEvents(r *http.Request, args *rpccommon.InstanceRequest, resp *[]qmp.Event) error {
	if args.VM.R != nil {
		ee, found, err := h.mon.FindEvents(args.Name, "", 0)
		if err != nil {
			return err
		}
		if found {
			*resp = ee
		} else {
			*resp = make([]qmp.Event, 0, 0)
		}
	}

	return nil
}

func (h *rpcHandler) GetBrief(r *http.Request, args *rpccommon.BriefRequest, resp *[]*rpccommon.VMSummary) error {
	var vmnames []string

	if args != nil && len(args.Names) > 0 {
		vmnames = args.Names
	} else {
		if nn, err := getVMNames(); err == nil {
			vmnames = nn
		} else {
			return err
		}
	}

	brief := make([]*rpccommon.VMSummary, 0, len(vmnames))

	if len(vmnames) == 0 {
		*resp = brief
		return nil
	}

	resultChan := make(chan *rpccommon.VMSummary, 10)

	go func() {
		jobChan := make(chan string)
		for i := 0; i < runtime.NumCPU()*2; i++ {
			go func() {
				for name := range jobChan {
					resultChan <- h.getVMSummary(name)
				}

			}()
		}
		for _, n := range vmnames {
			jobChan <- n
		}
		close(jobChan)
	}()

	for i := 0; i < len(vmnames); i++ {
		brief = append(brief, <-resultChan)
	}
	close(resultChan)

	// Sort by virt.machine name
	name := func(p1, p2 *rpccommon.VMSummary) bool {
		return p1.Name < p2.Name
	}

	By(name).Sort(brief)

	*resp = brief

	return nil
}

func (h *rpcHandler) getVMSummary(name string) *rpccommon.VMSummary {
	mon, _ := h.mon.Get(name)

	vm, err := kvmrun.GetVirtMachine(name, mon)
	if err != nil {
		fmt.Printf("getVMSummary error 1: %#v (%T)\n", err, err)
		return &rpccommon.VMSummary{Name: name, HasError: true}
	}

	s := rpccommon.VMSummary{Name: name}

	vmi := vm.C
	if vm.R != nil {
		vmi = vm.R
		if t, err := ps.GetLifeTime(vm.R.Pid()); err == nil {
			s.LifeTime = t
		} else {
			s.LifeTime = 0
		}
	}

	s.Pid = vmi.Pid()

	s.MemActual = vmi.GetActualMem()
	s.MemTotal = vmi.GetTotalMem()
	s.CPUActual = vmi.GetActualCPUs()
	s.CPUTotal = vmi.GetTotalCPUs()
	s.CPUQuota = vmi.GetCPUQuota()

	switch st, err := h.getVMStatus(vm); {
	case err == nil:
		s.State = st
	default:
		fmt.Printf("getVMSummary error 2: %#v (%T)\n", err, err)
		s.HasError = true
	}

	return &s
}

type briefSorter struct {
	brief []*rpccommon.VMSummary
	by    func(p1, p2 *rpccommon.VMSummary) bool
}

func (s *briefSorter) Len() int {
	return len(s.brief)
}
func (s *briefSorter) Swap(i, j int) {
	s.brief[i], s.brief[j] = s.brief[j], s.brief[i]
}
func (s *briefSorter) Less(i, j int) bool {
	return s.by(s.brief[i], s.brief[j])
}

type By func(p1, p2 *rpccommon.VMSummary) bool

func (by By) Sort(brief []*rpccommon.VMSummary) {
	bs := &briefSorter{
		brief: brief,
		by:    by,
	}
	sort.Sort(bs)
}

func getVMNames() ([]string, error) {
	files, err := ioutil.ReadDir(kvmrun.CONFDIR)
	if err != nil {
		return nil, err
	}

	vmnames := make([]string, 0, len(files))

	for _, f := range files {
		conffile := filepath.Join(kvmrun.CONFDIR, f.Name(), "config")
		if _, err := os.Stat(conffile); err == nil {
			vmnames = append(vmnames, f.Name())
		}
	}

	return vmnames, nil
}
