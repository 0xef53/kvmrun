package types

import (
	"fmt"
	"strings"
)

type CloudInitDriver struct {
	driver string
}

func DefaultCloudInitDriver() *CloudInitDriver {
	return &CloudInitDriver{"ide-cd"}
}

func (t *CloudInitDriver) Set(value string) error {
	driverName := strings.ToLower(strings.TrimSpace(value))

	switch driverName {
	case "floppy", "ide-cd":
	default:
		return fmt.Errorf("unknown driver name: %s", value)
	}

	t.driver = driverName

	return nil
}

func (t CloudInitDriver) String() string {
	return t.driver
}

func (t CloudInitDriver) Value() string {
	return t.driver
}
