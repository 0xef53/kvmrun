package flag_types

import (
	"fmt"
	"net"
	"strings"

	pb_types "github.com/0xef53/kvmrun/api/types/v2"
)

type NetIfaceHwAddr struct {
	hwaddr net.HardwareAddr
}

func (t *NetIfaceHwAddr) Set(value string) error {
	hwaddr, err := net.ParseMAC(value)
	if err != nil {
		return err
	}

	t.hwaddr = hwaddr

	return nil
}

func (t NetIfaceHwAddr) String() string {
	return t.hwaddr.String()
}

func (t NetIfaceHwAddr) Get() interface{} {
	return t.hwaddr
}

func DefaultNetIfaceDriver() *NetIfaceDriver {
	return &NetIfaceDriver{driver: pb_types.NetIfaceDriver_VIRTIO_NET_PCI}
}

type NetIfaceDriver struct {
	driver pb_types.NetIfaceDriver
}

func (t *NetIfaceDriver) Set(value string) error {
	driverName := strings.ReplaceAll(strings.ToUpper(value), "-", "_")

	v, ok := pb_types.NetIfaceDriver_value[driverName]
	if !ok {
		return fmt.Errorf("unknown net interface driver name: %s", value)
	}

	t.driver = pb_types.NetIfaceDriver(v)

	return nil
}

func (t NetIfaceDriver) String() string {
	return strings.ReplaceAll(strings.ToLower(t.driver.String()), "_", "-")
}

func (t NetIfaceDriver) Get() interface{} {
	return t.driver
}

type NetIfaceLinkState struct {
	state uint16
}

func (t *NetIfaceLinkState) Set(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "up":
		t.state = 1
	case "down":
		t.state = 2
	default:
		return fmt.Errorf("unknown link state value: %s", value)
	}

	return nil
}

func (t NetIfaceLinkState) String() string {
	if t.state == 1 {
		return "UP"
	}

	return "DOWN"
}

func (t NetIfaceLinkState) Get() interface{} {
	return t.state
}
