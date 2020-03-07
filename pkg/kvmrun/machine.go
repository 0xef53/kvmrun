package kvmrun

import (
	qmp "github.com/0xef53/go-qmp/v2"

	"github.com/0xef53/kvmrun/pkg/runsv"
)

type Instance interface {
	Clone() Instance

	Name() string
	Status() (string, error)

	GetActualMem() int
	GetTotalMem() int
	SetTotalMem(int) error
	SetActualMem(int) error

	GetActualCPUs() int
	GetTotalCPUs() int
	GetCPUQuota() int
	GetCPUModel() string
	SetActualCPUs(int) error
	SetTotalCPUs(int) error
	SetCPUQuota(int) error
	SetCPUModel(string) error

	GetDisks() Disks
	ResizeDisk(string) error
	AppendDisk(Disk) error
	InsertDisk(Disk, int) error
	RemoveDisk(string) error
	SetDiskReadIops(string, int) error
	SetDiskWriteIops(string, int) error
	RemoveDiskBitmap(string) error

	GetNetIfaces() NetIfaces
	AppendNetIface(NetIface) error
	RemoveNetIface(string) error
	SetNetIfaceUpScript(string, string) error
	SetNetIfaceDownScript(string, string) error
	SetNetIfaceLinkUp(string) error
	SetNetIfaceLinkDown(string) error

	GetChannels() Channels
	AppendChannel(VirtioChannel) error
	RemoveChannel(string) error

	GetKernelImage() string
	GetKernelCmdline() string
	GetKernelInitrd() string
	GetKernelModiso() string
	RemoveKernelConf() error
	SetKernelImage(string) error
	SetKernelCmdline(string) error
	SetKernelInitrd(string) error
	SetKernelModiso(string) error

	SetVNCPassword(string) error

	Uid() int
	Pid() int
	GetMachineType() string
	SetMachineType(string) error
	Save() error
	IsIncoming() bool
}

type VirtMachine struct {
	Name        string   `json:"name"`
	C           Instance `json:"conf"`
	R           Instance `json:"run,omitempty"`
	isInmigrate bool     `json:"-"`
}

func GetVirtMachine(vmname string, mon *qmp.Monitor) (*VirtMachine, error) {
	vmc, err := GetInstanceConf(vmname)
	if err != nil {
		return nil, err
	}

	vmr, err := GetInstanceQemu(vmname, mon)
	switch err.(type) {
	case nil:
	case *NotRunningError:
	default:
		return nil, err
	}

	vm := VirtMachine{
		Name: vmname,
		C:    vmc,
		R:    vmr,
	}

	return &vm, nil
}

func (vm VirtMachine) Clone() *VirtMachine {
	x := VirtMachine{
		Name: vm.Name,
		C:    vm.C.Clone(),
		R:    vm.R.Clone(),
	}

	return &x
}

func (vm *VirtMachine) Status() (string, error) {
	vmi := vm.C
	if vm.R != nil {
		vmi = vm.R
	}

	// Trying to get the special status of instance.
	// Exiting on success.
	if st, err := vmi.Status(); err == nil {
		switch st {
		case "incoming", "inmigrate", "migrated":
			return st, nil
		}
	} else {
		return "", err
	}

	// And finally looking at the status of a service
	if !runsv.IsEnabled(vm.Name) {
		return "inactive", nil
	}
	serviceState, err := runsv.GetWantState(vm.Name)
	if err != nil {
		return "", err
	}

	return serviceState, nil
}
