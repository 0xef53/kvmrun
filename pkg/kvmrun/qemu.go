package kvmrun

import (
	"fmt"
)

type QemuVersion int

func (v QemuVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v/10000, (v%10000)/100, (v%10000)%100)
}
