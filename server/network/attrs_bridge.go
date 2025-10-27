package network

import (
	"fmt"
	"strings"
)

type NetworkSchemeAttrs_Bridge struct {
	commonAttrs

	BridgeInterface string `json:"bridge_interface"`
}

func (x *NetworkSchemeAttrs_Bridge) Validate(strict bool) error {
	if err := x.commonAttrs.Validate(strict); err != nil {
		return err
	}

	x.BridgeInterface = strings.TrimSpace(x.BridgeInterface)

	if len(x.BridgeInterface) == 0 {
		return fmt.Errorf("empty bridge.bridge_interface value")
	}

	return nil
}

func (x *NetworkSchemeAttrs_Bridge) Properties() *SchemeProperties {
	p := x.commonAttrs.Properties()

	p.SchemeType = Scheme_BRIDGE

	p.Set("bridge_interface", x.BridgeInterface)

	return p
}
