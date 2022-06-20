package types

import (
	"fmt"
	"net"
	"strings"

	pb_types "github.com/0xef53/kvmrun/api/types"
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

func (t NetIfaceHwAddr) Value() net.HardwareAddr {
	return t.hwaddr
}

type NetIfaceDriver struct {
	driver pb_types.NetIfaceDriver
}

func DefaultNetIfaceDriver() *NetIfaceDriver {
	return &NetIfaceDriver{pb_types.NetIfaceDriver_VIRTIO_NET_PCI}
}

func (t *NetIfaceDriver) Set(value string) error {
	driverName := strings.ReplaceAll(strings.ToUpper(value), "-", "_")

	v, ok := pb_types.NetIfaceDriver_value[driverName]
	if !ok {
		return fmt.Errorf("unknown driver name: %s", value)
	}

	t.driver = pb_types.NetIfaceDriver(v)

	return nil
}

func (t NetIfaceDriver) String() string {
	return strings.ReplaceAll(strings.ToLower(t.driver.String()), "_", "-")
}

func (t NetIfaceDriver) Value() pb_types.NetIfaceDriver {
	return t.driver
}

type NetIfaceLinkState struct {
	state pb_types.NetIfaceLinkState
}

func (t *NetIfaceLinkState) Set(value string) error {
	v, ok := pb_types.NetIfaceLinkState_value[strings.ToUpper(value)]
	if !ok {
		return fmt.Errorf("unknown link state value: %s", value)
	}

	t.state = pb_types.NetIfaceLinkState(v)

	return nil
}

func (t NetIfaceLinkState) String() string {
	return strings.ToLower(t.state.String())
}

func (t NetIfaceLinkState) Value() pb_types.NetIfaceLinkState {
	return t.state
}
