package types

import (
	"fmt"
	"strings"

	pb_types "github.com/0xef53/kvmrun/api/types"
)

type CdromDriver struct {
	driver pb_types.CdromDriver
}

func DefaultCdromDriver() *CdromDriver {
	return &CdromDriver{pb_types.CdromDriver_IDE_CD}
}

func (t *CdromDriver) Set(value string) error {
	driverName := strings.ReplaceAll(strings.ToUpper(value), "-", "_")

	v, ok := pb_types.CdromDriver_value[driverName]
	if !ok {
		return fmt.Errorf("unknown driver name: %s", value)
	}

	t.driver = pb_types.CdromDriver(v)

	return nil
}

func (t CdromDriver) String() string {
	return strings.ReplaceAll(strings.ToLower(t.driver.String()), "_", "-")
}

func (t CdromDriver) Value() pb_types.CdromDriver {
	return t.driver
}
