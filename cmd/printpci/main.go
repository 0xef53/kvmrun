package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/0xef53/kvmrun/internal/pci"
	"github.com/0xef53/kvmrun/kvmrun"

	pb_types "github.com/0xef53/kvmrun/api/types/v2"
)

func pciDeviceToProto(dev *pci.Device) *pb_types.PCIDevice {
	return &pb_types.PCIDevice{
		Addr:          dev.String(),
		Driver:        dev.CurrentDriver(),
		Enabled:       dev.Enabled(),
		Multifunction: dev.HasMultifunctionFeature(),
		Class:         dev.Class(),
		Vendor:        uint32(dev.Vendor()),
		Device:        uint32(dev.Device()),
		ClassName:     dev.ClassName(),
		SubclassName:  dev.SubclassName(),
		VendorName:    dev.VendorName(),
		DeviceName:    dev.DeviceName(),
	}
}

func pciDeviceListToProto(devices []*pci.Device) []*pb_types.PCIDevice {
	protos := make([]*pb_types.PCIDevice, 0, len(devices))

	for _, dev := range devices {
		protos = append(protos, pciDeviceToProto(dev))
	}

	return protos
}

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	var devices []*pb_types.PCIDevice

	if v, err := pci.DeviceList(); err == nil {
		devices = pciDeviceListToProto(v)
	} else {
		return err
	}

	// Try to check reserved devices and their holders
	vmnames, err := getMachineNames()
	if err != nil && !os.IsNotExist(err) {
		return err
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
		if vmname, ok := holders[devices[idx].Addr]; ok {
			devices[idx].Reserved = true
			devices[idx].Holder = vmname
		}
	}

	b, err := json.MarshalIndent(devices, "", "    ")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("%s\n", string(b))

	return nil
}

func getMachineNames() ([]string, error) {
	files, err := os.ReadDir(kvmrun.CONFDIR)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(files))

	for _, f := range files {
		conffile := filepath.Join(kvmrun.CONFDIR, f.Name(), "config")

		if _, err := os.Stat(conffile); err == nil {
			names = append(names, f.Name())
		}
	}

	return names, nil
}
