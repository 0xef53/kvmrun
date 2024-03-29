package pci

import (
	"fmt"
	"strconv"
	"strings"
)

type Address struct {
	Domain   uint16
	Bus      uint8
	Device   uint8
	Function uint8
}

func AddressFromHex(s string) (*Address, error) {
	var domain uint16
	var bus, device, function uint8

	switch ff := strings.Split(string(s), ":"); len(ff) {
	case 3:
		if ff[0] = strings.TrimSpace(ff[0]); len(ff[0]) > 0 {
			if v, err := strconv.ParseUint(ff[0], 16, 16); err == nil {
				domain = uint16(v)
			} else {
				return nil, err
			}
		} else {
			domain = 0
		}
		ff = ff[1:]
		fallthrough
	case 2:
		if v, err := strconv.ParseUint(ff[0], 16, 8); err == nil {
			bus = uint8(v)
		} else {
			return nil, err
		}
		switch ff2 := strings.Split(ff[1], "."); len(ff2) {
		case 2:
			if v, err := strconv.ParseUint(ff2[1], 16, 8); err == nil {
				if v > 7 {
					return nil, fmt.Errorf("a function cannot be a number larger than 0x7")
				}
				function = uint8(v)
			} else {
				return nil, err
			}
			fallthrough
		case 1:
			if v, err := strconv.ParseUint(ff2[0], 16, 8); err == nil {
				if v > 31 {
					return nil, fmt.Errorf("a slot cannot be a number larger than 0x1f")
				}
				device = uint8(v)
			} else {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("bad pci address format: want '[domain:]bus:device.function', given '%s'", s)
	}

	return &Address{
		Domain:   domain,
		Bus:      bus,
		Device:   device,
		Function: function,
	}, nil
}

func (a *Address) String() string {
	return fmt.Sprintf("%.4x:%.2x:%.2x.%x", a.Domain, a.Bus, a.Device, a.Function)
}

func (a *Address) Prefix() string {
	return fmt.Sprintf("%.4x:%.2x:%.2x", a.Domain, a.Bus, a.Device)
}
