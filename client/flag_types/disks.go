package flag_types

import (
	"fmt"
	"strings"

	pb_types "github.com/0xef53/kvmrun/api/types/v2"
)

func DefaultDiskDriver() *DiskDriver {
	return &DiskDriver{driver: pb_types.DiskDriver_VIRTIO_BLK_PCI}
}

type DiskDriver struct {
	driver pb_types.DiskDriver
}

func (t *DiskDriver) Set(value string) error {
	driverName := strings.ReplaceAll(strings.ToUpper(value), "-", "_")

	v, ok := pb_types.DiskDriver_value[driverName]
	if !ok {
		return fmt.Errorf("unknown disk driver name: %s", value)
	}

	t.driver = pb_types.DiskDriver(v)

	return nil
}

func (t DiskDriver) String() string {
	return strings.ReplaceAll(strings.ToLower(t.driver.String()), "_", "-")
}

func (t DiskDriver) Get() interface{} {
	return t.driver
}
