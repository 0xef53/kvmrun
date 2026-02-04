package machine

import (
	"runtime"
	"sort"
	"time"

	"github.com/0xef53/kvmrun/kvmrun"
)

type MachineInfo struct {
	M *kvmrun.Machine

	State    kvmrun.InstanceState
	LifeTime time.Duration
}

func (s *Server) GetList(names ...string) ([]*MachineInfo, error) {
	validNames, err := s.MachineGetNames(names...)
	if err != nil {
		return nil, err
	}

	if len(validNames) == 0 {
		return nil, nil
	}

	get := func(name string) *MachineInfo {
		vm, err := s.MachineGet(name, true)
		if err != nil {
			return &MachineInfo{M: &kvmrun.Machine{Name: name}, State: kvmrun.StateNoState}
		}

		vmstate, err := s.MachineGetStatus(vm)
		if err != nil {
			return &MachineInfo{M: &kvmrun.Machine{Name: name}, State: kvmrun.StateCrashed}
		}

		t, err := s.MachineGetLifeTime(vm)
		if err != nil {
			return &MachineInfo{M: &kvmrun.Machine{Name: name}, State: kvmrun.StateCrashed}
		}

		return &MachineInfo{M: vm, State: vmstate, LifeTime: t}
	}

	vmlist := make([]*MachineInfo, 0, len(validNames))
	results := make(chan *MachineInfo, runtime.NumCPU())

	go func() {
		jobs := make(chan string)

		// Start workers
		for i := 0; i < runtime.NumCPU(); i++ {
			go func() {
				for name := range jobs {
					results <- get(name)
				}
			}()
		}

		for _, name := range validNames {
			jobs <- name
		}

		close(jobs)
	}()

	// Collect results
	for i := 0; i < len(validNames); i++ {
		vmlist = append(vmlist, <-results)
	}
	close(results)

	// Sort by virtual machine name
	sort.Slice(vmlist, func(i, j int) bool {
		return vmlist[i].M.Name < vmlist[j].M.Name
	})

	return vmlist, nil
}
