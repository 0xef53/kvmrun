package network

import (
	"fmt"
	"strings"
)

type NetworkSchemeAttrs_VxLAN struct {
	commonAttrs

	VNI           uint32 `json:"vni"`
	BindInterface string `json:"bind_interface"`
}

func (x *NetworkSchemeAttrs_VxLAN) Validate(strict bool) error {
	if err := x.commonAttrs.Validate(strict); err != nil {
		return err
	}

	x.BindInterface = strings.TrimSpace(x.BindInterface)

	if len(x.BindInterface) == 0 {
		return fmt.Errorf("empty vxlan.bind_interface value")
	}

	maxVNI := uint32(1<<(24) - 1)

	if !(x.VNI > 0 && x.VNI < maxVNI) {
		return fmt.Errorf("invalid vxlan.vni value: must be greater than 0 and less or equal %d", maxVNI)
	}

	return nil
}

func (x *NetworkSchemeAttrs_VxLAN) Properties() *SchemeProperties {
	p := x.commonAttrs.Properties()

	p.SchemeType = Scheme_VXLAN

	p.Set("vni", x.VNI)
	p.Set("bind_interface", x.BindInterface)

	return p
}
