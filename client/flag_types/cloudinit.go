package flag_types

import (
	"fmt"
	"strings"

	pb_types "github.com/0xef53/kvmrun/api/types/v2"
)

type CloudInitDriver struct {
	driver pb_types.CloudInitDriver
}

func DefaultCloudInitDriver() *CloudInitDriver {
	return &CloudInitDriver{driver: pb_types.CloudInitDriver_CI_IDE_CD}
}

func (t *CloudInitDriver) Set(value string) error {
	driverName := "CI_" + strings.ReplaceAll(strings.ToUpper(value), "-", "_")

	v, ok := pb_types.CloudInitDriver_value[driverName]
	if !ok {
		return fmt.Errorf("unknown cloud-init driver name: %s", value)
	}

	t.driver = pb_types.CloudInitDriver(v)

	return nil
}

func (t CloudInitDriver) String() string {
	return strings.ReplaceAll(strings.TrimPrefix(strings.ToLower(t.driver.String()), "ci_"), "_", "-")
}

func (t CloudInitDriver) Get() interface{} {
	return t.driver
}
