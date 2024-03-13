package pci

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Address struct {
	Domain   uint16
	Bus      uint8
	Slot     uint8
	Function uint8
}

func AddressFromHex(s string) (*Address, error) {
	var domain uint16
	var bus, slot, function uint8

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
				slot = uint8(v)
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
		Slot:     slot,
		Function: function,
	}, nil
}

func (a *Address) String() string {
	return fmt.Sprintf("%.4x:%.2x:%.2x.%x", a.Domain, a.Bus, a.Slot, a.Function)
}

func (a *Address) Prefix() string {
	return fmt.Sprintf("%.4x:%.2x:%.2x", a.Domain, a.Bus, a.Slot)
}

type Device struct {
	addr *Address
}

func NewDevice(s string) (*Device, error) {
	addr, err := AddressFromHex(s)
	if err != nil {
		return nil, err
	}

	return &Device{addr}, nil
}

func (d *Device) String() string {
	return d.addr.String()
}

func (d *Device) Prefix() string {
	return d.addr.Prefix()
}

func (d *Device) FullPath() string {
	return filepath.Join("/sys/bus/pci/devices", d.addr.String())
}

func (d *Device) Domain() uint16 {
	return d.addr.Domain
}

func (d *Device) Bus() uint8 {
	return d.addr.Bus
}

func (d *Device) Slot() uint8 {
	return d.addr.Slot
}

func (d *Device) Function() uint8 {
	return d.addr.Function
}

func (d *Device) IsEnabled() (bool, error) {
	data, err := os.ReadFile(filepath.Join(d.FullPath(), "enable"))
	if err != nil {
		return false, err
	}

	if strings.TrimSpace(string(data)) == "1" {
		return true, nil
	}

	return false, nil
}

func (d *Device) HasMultifunctionFeature() (bool, error) {
	data, err := os.ReadFile(filepath.Join(d.FullPath(), "config"))
	if err != nil {
		return false, err
	}

	// Check the 7th bit in the HeaderType
	if len(data) > 0 && (data[0]>>7)&1 == 1 {
		return true, nil
	}

	return false, nil
}

func (d *Device) GetAllFunctions() ([]uint8, error) {
	if d.addr.Function != 0 {
		return nil, fmt.Errorf("non general device: %s", d.addr.String())
	}

	if ok, err := d.HasMultifunctionFeature(); err == nil {
		if !ok {
			return nil, fmt.Errorf("multifunction is not supported: %s", d.addr.String())
		}
	} else {
		return nil, err
	}

	files, err := os.ReadDir(d.FullPath())
	if err != nil {
		return nil, err
	}

	prefix := d.addr.Prefix()

	functions := make([]uint8, 0, 7)

	for _, f := range files {
		if strings.HasPrefix(f.Name(), "consumer:pci:"+prefix) {
			if addr, err := AddressFromHex(strings.TrimPrefix(f.Name(), "consumer:pci:"+prefix)); err == nil {
				functions = append(functions, addr.Function)
			} else {
				return nil, fmt.Errorf("unexpected: %w", err)
			}
		}
	}

	return functions, nil
}

func (d *Device) GetCurrentDriver() (string, error) {
	s, err := filepath.EvalSymlinks(filepath.Join(d.FullPath(), "driver"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	return filepath.Base(s), nil
}

func (d *Device) AssignDriver(n string) error {
	if n = strings.TrimSpace(n); len(n) == 0 {
		return fmt.Errorf("empty driver name")
	}

	driver, err := d.GetCurrentDriver()
	if err != nil {
		return fmt.Errorf("cannot determine the current driver: %w", err)
	}

	if driver == n {
		return nil
	}

	if len(driver) > 0 {
		if err := os.WriteFile(filepath.Join(d.FullPath(), "driver/unbind"), []byte(d.String()), 0200); err != nil {
			return fmt.Errorf("failed to unbind: %w", err)
		}
	}

	if err := os.WriteFile(filepath.Join("/sys/bus/pci/drivers", n, "new_id"), []byte(d.String()), 0200); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("failed to assign driver: is %s loaded?", n)
		}
		return fmt.Errorf("failed to assign driver: %w", err)
	}

	driver, err = d.GetCurrentDriver()
	if err != nil {
		return fmt.Errorf("cannot check the new driver: %w", err)
	}

	if driver != n {
		return fmt.Errorf("failed to assign driver: run 'dmesg' for more details")
	}

	return nil
}

func (d *Device) UnbindDriver() error {
	driver, err := d.GetCurrentDriver()
	if err != nil {
		return fmt.Errorf("cannot determine the current driver: %w", err)
	}

	if len(driver) > 0 {
		if err := os.WriteFile(filepath.Join(d.FullPath(), "driver/unbind"), []byte(d.String()), 0200); err != nil {
			return fmt.Errorf("failed to unbind: %w", err)
		}
	}

	return nil
}
