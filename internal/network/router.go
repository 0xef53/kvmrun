package network

import (
//"github.com/vishvananda/netlink"
)

type RouterDeviceAttrs struct {
	MTU uint32
}

func ConfigureRouter(linkname string, attrs *RouterDeviceAttrs) error {
	return nil
}

func DeconfigureRouter(linkname string) error {
	return nil
}
