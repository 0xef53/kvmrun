package hardware

import (
	"github.com/0xef53/kvmrun/internal/pci"
	"github.com/0xef53/kvmrun/kvmrun"
)

type PCIDeviceInfo struct {
	*pci.Device

	Reserved bool   `json:"reserved"`
	Holder   string `jdon:"holder"`
}

func (s *Server) GetPCIDeviceList() ([]*PCIDeviceInfo, error) {
	var devices []*PCIDeviceInfo

	if pcidevs, err := pci.DeviceList(); err == nil {
		devices = make([]*PCIDeviceInfo, 0, len(pcidevs))

		for _, v := range pcidevs {
			devices = append(devices, &PCIDeviceInfo{Device: v})
		}
	} else {
		return nil, err
	}

	// Try to check reserved devices and their holders
	vmnames, err := s.MachineGetNames()
	if err != nil {
		return nil, err
	}

	holders := make(map[string]string)

	for _, vmname := range vmnames {
		vmconf, err := kvmrun.GetInstanceConf(vmname)
		if err != nil {
			// no problem, just continue
			continue
		}

		for _, dev := range vmconf.HostDeviceGetList() {
			if dev.BackendAddr != nil {
				holders[dev.BackendAddr.String()] = vmname
			}
		}
	}

	for idx := range devices {
		if vmname, ok := holders[devices[idx].Device.String()]; ok {
			devices[idx].Reserved = true
			devices[idx].Holder = vmname
		}
	}

	return devices, nil
}
