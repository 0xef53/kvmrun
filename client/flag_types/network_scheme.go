package flag_types

import (
	"fmt"
	"strings"

	"github.com/0xef53/kvmrun/server/network"
)

type NetworkSchemeType struct {
	Type network.SchemeType
}

func DefaultNetworkSchemeType() *NetworkSchemeType {
	return &NetworkSchemeType{Type: network.Scheme_MANUAL}
}

func (t *NetworkSchemeType) Set(value string) error {
	value = strings.TrimSpace(strings.ToLower(value))

	if v := network.SchemeTypeValue(value); v != network.Scheme_UNKNOWN {
		t.Type = v
	} else {
		return fmt.Errorf("unknown network scheme type: %s", value)
	}

	return nil
}

func (t NetworkSchemeType) String() string {
	return t.Type.String()
}

func (t NetworkSchemeType) Get() interface{} {
	return t.Type
}
