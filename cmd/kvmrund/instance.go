package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	qmp "github.com/0xef53/go-qmp/v2"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	"github.com/0xef53/kvmrun/pkg/ps"
	"github.com/0xef53/kvmrun/pkg/pwd"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
	"github.com/0xef53/kvmrun/pkg/runsv"
)

func (x *RPC) CreateConfInstance(r *http.Request, args *rpccommon.NewInstanceRequest, resp *struct{}) error {
	if len(args.Launcher) > 0 {
		st, err := os.Stat(args.Launcher)
		if err != nil {
			return err
		}
		if !(st.Mode().IsRegular() && st.Mode()&0100 > 0) {
			return fmt.Errorf("Not an executable file: %s", args.Launcher)
		}
	}

	switch err := kvmrun.CreateService(args.Name); {
	case os.IsExist(err):
		return fmt.Errorf("Already exists: %s", args.Name)
	case err != nil:
		return err
	}

	if len(args.Launcher) > 0 {
		runfile := filepath.Join(kvmrun.VMCONFDIR, args.Name, "run")
		if err := os.Remove(runfile); err != nil {
			return err
		}
		if err := os.Symlink(args.Launcher, runfile); err != nil {
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

	return runsv.Enable(args.Name, false)
}

func (x *RPC) CreateConfInstanceFromManifest(r *http.Request, args *rpccommon.NewManifestInstanceRequest, port *int) error {
	var vm IncomingVM

	if err := json.Unmarshal(args.Manifest, &vm); err != nil {
		return err
	}

	if err := kvmrun.CreateService(vm.Name); err != nil {
		return err
	}
	if err := vm.C.Save(); err != nil {
		return err
	}

	// Run file
	if len(args.Launcher) > 0 && args.Launcher != "/usr/lib/kvmrun/launcher" {
		launcher := filepath.Join(kvmrun.VMCONFDIR, vm.Name, "run")
		if err := os.Remove(launcher); err != nil {
			return err
		}
		if err := os.Symlink(args.Launcher, launcher); err != nil {
			return err
		}
	}

	// Finish file
	if len(args.Finisher) > 0 && args.Finisher != "/usr/lib/kvmrun/finisher" {
		finisher := filepath.Join(kvmrun.VMCONFDIR, vm.Name, "finish")
		if err := os.Remove(finisher); err != nil {
			return err
		}
		if err := os.Symlink(args.Finisher, finisher); err != nil {
			return err
		}
	}

	// Extra files
	if args.ExtraFiles != nil {
		for fname, content := range args.ExtraFiles {
			if err := ioutil.WriteFile(filepath.Join(kvmrun.VMCONFDIR, vm.Name, fname), content, 0644); err != nil {
				return err
			}
		}
	}

	if _, err := pwd.CreateUser(vm.Name); err != nil {
		return err
	}

	if err := runsv.Enable(vm.Name, false); err != nil {
		return err
	}

	return nil
}

func (x *RPC) RemoveConfInstance(r *http.Request, args *rpccommon.InstanceRequest, port *int) error {
	ioutil.WriteFile(filepath.Join(kvmrun.VMCONFDIR, args.Name, "down"), []byte{}, 0644)
	runsv.SendSignal(args.Name, "x")
	pwd.RemoveUser(args.Name)
	return kvmrun.RemoveService(args.Name)
}

func (x *RPC) GetInstanceJSON(r *http.Request, args *rpccommon.InstanceRequest, resp *[]byte) error {
	b, err := json.MarshalIndent(args.VM, "", "    ")
	if err != nil {
		return err
	}
	*resp = b

	return nil
}

func (x *RPC) IsInstanceRunning(r *http.Request, args *rpccommon.InstanceRequest, resp *bool) error {
	if args.VM.R != nil {
		*resp = true
	}

	return nil
}

func (x *RPC) IsConfExist(r *http.Request, args *rpccommon.VMNameRequest, resp *bool) error {
	runfile := filepath.Join(kvmrun.VMCONFDIR, args.Name, "run")

	switch _, err := os.Stat(runfile); {
	case err == nil:
		*resp = true
	case os.IsNotExist(err):
	default:
		return err
	}

	return nil
}

func (x *RPC) GetInstanceNames(r *http.Request, args *struct{}, resp *[]string) error {
	nn, err := getVMNames()
	if err != nil {
		return err
	}

	*resp = nn

	return nil
}

func (x *RPC) StopQemuInstance(r *http.Request, args *rpccommon.VMNameRequest, resp *struct{}) error {
	if err := ioutil.WriteFile(filepath.Join(kvmrun.VMCONFDIR, args.Name, "down"), []byte{}, 0644); err != nil {
		return err
	}
	return runsv.SendSignal(args.Name, "x")
}

func (x *RPC) SendCont(r *http.Request, args *rpccommon.VMNameRequest, resp *struct{}) error {
	return QPool.Run(args.Name, qmp.Command{"cont", nil}, nil)
}

func (x *RPC) SendStop(r *http.Request, args *rpccommon.VMNameRequest, resp *struct{}) error {
	return QPool.Run(args.Name, qmp.Command{"stop", nil}, nil)
}

func (x *RPC) GetQemuEvents(r *http.Request, args *rpccommon.InstanceRequest, resp *[]qmp.Event) error {
	if args.VM.R != nil {
		ee, found, err := QPool.FindEvents(args.Name, "", 0)
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

func (x *RPC) GetBrief(r *http.Request, args *rpccommon.BriefRequest, resp *[]*rpccommon.VMSummary) error {
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
			go briefWorker(jobChan, resultChan)
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

func getVMNames() ([]string, error) {
	dir, err := os.Open(kvmrun.VMCONFDIR)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	files, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}

	vmnames := make([]string, 0, len(files))

	for _, f := range files {
		if f.IsDir() {
			vmnames = append(vmnames, f.Name())
		}
	}

	return vmnames, nil
}

func briefWorker(jobChan <-chan string, resultChan chan<- *rpccommon.VMSummary) {
	for name := range jobChan {
		resultChan <- getVMSummary(name)
	}
}

func getVMSummary(name string) *rpccommon.VMSummary {
	mon, _ := QPool.Get(name)

	vm, err := kvmrun.GetVirtMachine(name, mon)
	if err != nil {
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

	switch st, err := vm.Status(); {
	case err == nil:
		s.State = st
	default:
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
