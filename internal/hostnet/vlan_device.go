package hostnet

type VlanDeviceAttrs struct {
	VlanID uint32
	MTU    uint32
	Parent string
}

func VlanPortConfigure(linkname string, attrs *VlanDeviceAttrs, secondStage bool) error {
	if secondStage {
		// no second stage for this scheme
		return nil
	}

	return nil
}

func VlanPortDeconfigure(linkname string, vlan_id uint32) error {
	return nil
}
