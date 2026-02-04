package flag_types

import (
	"fmt"
	"strings"

	pb_types "github.com/0xef53/kvmrun/api/types/v2"
)

type InputDeviceType struct {
	Type pb_types.InputDeviceType
}

func DefaultInputDeviceType() *InputDeviceType {
	return &InputDeviceType{pb_types.InputDeviceType_USB_TABLET}
}

func (t *InputDeviceType) Set(value string) error {
	typeName := strings.ReplaceAll(strings.ToUpper(value), "-", "_")

	v, ok := pb_types.InputDeviceType_value[typeName]
	if !ok {
		return fmt.Errorf("unknown device type: %s", value)
	}

	t.Type = pb_types.InputDeviceType(v)

	return nil
}

func (t InputDeviceType) String() string {
	return strings.ReplaceAll(strings.ToLower(t.Type.String()), "_", "-")
}

func (t InputDeviceType) Get() interface{} {
	return t.Type
}
