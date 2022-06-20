package types

import (
	"fmt"
	"strings"

	pb_types "github.com/0xef53/kvmrun/api/types"
)

type DiskDriver struct {
	driver pb_types.DiskDriver
}

func DefaultDiskDriver() *DiskDriver {
	return &DiskDriver{pb_types.DiskDriver_VIRTIO_BLK_PCI}
}

func (t *DiskDriver) Set(value string) error {
	driverName := strings.ReplaceAll(strings.ToUpper(value), "-", "_")

	v, ok := pb_types.DiskDriver_value[driverName]
	if !ok {
		return fmt.Errorf("unknown driver name: %s", value)
	}

	t.driver = pb_types.DiskDriver(v)

	return nil
}

func (t DiskDriver) String() string {
	return strings.ReplaceAll(strings.ToLower(t.driver.String()), "_", "-")
}

func (t DiskDriver) Value() pb_types.DiskDriver {
	return t.driver
}
