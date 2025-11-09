package network

import (
	"fmt"
	"strings"
)

type NetworkSchemeAttrs_VLAN struct {
	commonAttrs

	VlanID          uint32 `json:"vlan_id"`
	ParentInterface string `json:"parent_interface"`
}

func (x *NetworkSchemeAttrs_VLAN) Validate(strict bool) error {
	if err := x.commonAttrs.Validate(strict); err != nil {
		return err
	}

	x.ParentInterface = strings.TrimSpace(x.ParentInterface)

	if len(x.ParentInterface) == 0 {
		return fmt.Errorf("empty vlan.parent_interface value")
	}

	maxVLAN := uint32(1<<(12) - 2)

	if !(x.VlanID > 0 && x.VlanID < maxVLAN) {
		return fmt.Errorf("invalid vlan.vlan_id value: must be greater than 0 and less or equal %d", maxVLAN)
	}

	return nil
}

func (x *NetworkSchemeAttrs_VLAN) Properties() *SchemeProperties {
	p := x.commonAttrs.Properties()

	p.SchemeType = Scheme_VLAN

	p.Set("vlan_id", x.VlanID)
	p.Set("parent_interface", x.ParentInterface)

	return p
}
