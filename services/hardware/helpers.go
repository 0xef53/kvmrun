package hardware

import (
	pb_types "github.com/0xef53/kvmrun/api/types"
	"github.com/0xef53/kvmrun/internal/pci"
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
