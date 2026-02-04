package flag_types

import (
	"fmt"
	"strings"
)

type StringBool struct {
	value bool
}

func NewStringBool() *StringBool {
	return &StringBool{value: false}
}

func (t *StringBool) Set(s string) error {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "on", "yes", "true":
		t.value = true
	case "0", "off", "no", "false":
		t.value = false
	default:
		return fmt.Errorf("incorrect value: %s", s)
	}

	return nil
}

func (t StringBool) String() string {
	if t.value {
		return "on"
	}

	return "off"
}

func (t StringBool) Get() interface{} {
	return t.value
}
