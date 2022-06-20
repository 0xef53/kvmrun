package kvmrun

import (
	qmp "github.com/0xef53/go-qmp/v2"
)

type Machine struct {
	Name string   `json:"name"`
	C    Instance `json:"conf"`
	R    Instance `json:"run,omitempty"`
}

func GetMachine(vmname string, mon *qmp.Monitor) (*Machine, error) {
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

	return &Machine{
		Name: vmname,
		C:    vmc,
		R:    vmr,
	}, nil
}
