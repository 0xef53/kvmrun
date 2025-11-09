package network

import (
	"fmt"
	"strings"
)

type NetworkSchemeAttrs_Routed struct {
	commonAttrs

	BindInterface string `json:"bind_interface"`
	InLimit       uint32 `json:"in_limit,omitempty"`
	OutLimit      uint32 `json:"out_limit,omitempty"`
}

func (x *NetworkSchemeAttrs_Routed) Validate(strict bool) error {
	if err := x.commonAttrs.Validate(strict); err != nil {
		return err
	}

	x.BindInterface = strings.TrimSpace(x.BindInterface)

	if len(x.BindInterface) == 0 {
		return fmt.Errorf("empty router.bind_interface value")
	}

	return nil
}

func (x *NetworkSchemeAttrs_Routed) Properties() *SchemeProperties {
	p := x.commonAttrs.Properties()

	p.SchemeType = Scheme_ROUTED

	p.Set("bind_interface", x.BindInterface)

	p.Set("in_limit", x.InLimit)
	p.Set("out_limit", x.OutLimit)

	return p
}
