package network

import (
//"github.com/vishvananda/netlink"
)

type VlanDeviceAttrs struct {
	VlanID uint32
	MTU    uint32
	Parent string
}

func ConfigureVlanPort(linkname string, attrs *VlanDeviceAttrs) error {
	return nil
}

func DeconfigureVlanPort(linkname string, vlan_id uint32) error {
	return nil
}
