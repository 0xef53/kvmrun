package pci

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

var ErrDeviceNotFound = errors.New("PCI device not found")

type Device struct {
	addr       *Address
	parent     *Device
	subdevices []*Device

	driver  string
	enabled bool

	classID  uint32
	vendorID uint16
	deviceID uint16

	className    string
	subclassName string
	vendorName   string
	deviceName   string

	multifunction bool

	mu sync.Mutex
}

func LookupDevice(hexaddr string) (*Device, error) {
	pcidev, err := func() (*Device, error) {
		pcidev, err := lookup(hexaddr)
		if err != nil {
			return nil, err
		}

		if pcidev.addr.Function > 0 {
			// Try to determine the parent device
			files, err := os.ReadDir(pcidev.FullPath())
			if err != nil {
				return nil, err
			}

			for _, f := range files {
				if strings.HasPrefix(f.Name(), "supplier:pci:") {
					if p, err := lookup(strings.TrimPrefix(f.Name(), "supplier:pci:"), pcidev); err == nil {
						pcidev.parent = p
					} else {
						return nil, err
					}
				}
			}
		}

		return pcidev, nil
	}()

	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrDeviceNotFound, hexaddr)
		}
		return nil, fmt.Errorf("read error (device: %s): %w", hexaddr, err)
	}

	return pcidev, nil
}

func lookup(hexaddr string, nested ...*Device) (*Device, error) {
	addr, err := AddressFromHex(hexaddr)
	if err != nil {
		return nil, err
	}

	pcidev := Device{addr: addr}

	if config, err := readHeadBytes(pcidev.FullPath(), "config", 64); err == nil {
		// Multifunction (offset 0x0E is the "Header Type")
		// The first function (function 0) must have bit 7 (0x80) of the Header Type (0x0E) register set.
		// See details on https://docs.pcbox-emu.xyz/en/latest/dev/api/pci.html.
		pcidev.multifunction = (config[0x0E] & 0x80) == 0x80
	} else {
		return nil, err
	}

	if s, err := readString(pcidev.FullPath(), "enable"); err == nil {
		pcidev.enabled = s == "1"
	} else {
		return nil, err
	}

	if n, err := readUint(pcidev.FullPath(), "class", 16, 32); err == nil {
		pcidev.classID = uint32(n)
		if v, ok := DB.FindClass(pcidev.ClassHex()); ok {
			pcidev.className = v.Name
			for _, sub := range v.Subclasses {
				pcidev.subclassName = sub.Name
			}
		}
	} else {
		return nil, err
	}

	if n, err := readUint(pcidev.FullPath(), "vendor", 16, 16); err == nil {
		pcidev.vendorID = uint16(n)
		if v, ok := DB.FindVendor(pcidev.VendorHex()); ok {
			pcidev.vendorName = v.Name
		}
	} else {
		return nil, err
	}

	if n, err := readUint(pcidev.FullPath(), "device", 16, 16); err == nil {
		pcidev.deviceID = uint16(n)
		if v, ok := DB.FindProduct(pcidev.VendorHex(), pcidev.DeviceHex()); ok {
			pcidev.deviceName = v.Name
		}
	} else {
		return nil, err
	}

	if s, err := filepath.EvalSymlinks(filepath.Join(pcidev.FullPath(), "driver")); err == nil {
		pcidev.driver = filepath.Base(s)
	} else {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if addr.Function == 0 && pcidev.multifunction {
		files, err := os.ReadDir(pcidev.FullPath())
		if err != nil {
			return nil, err
		}

		for _, f := range files {
			if strings.HasPrefix(f.Name(), "consumer:pci:") {
				hexaddr := strings.TrimPrefix(f.Name(), "consumer:pci:")

				var found bool

				for _, v := range nested {
					if v.String() == hexaddr {
						found = true
						pcidev.subdevices = append(pcidev.subdevices, v)
					}
				}

				if !found {
					if sub, err := lookup(hexaddr); err == nil {
						sub.parent = &pcidev
						pcidev.subdevices = append(pcidev.subdevices, sub)
					} else {
						return nil, err
					}
				}
			}
		}
	}

	return &pcidev, nil
}

func (d *Device) FullPath() string {
	return filepath.Join("/sys/bus/pci/devices", d.addr.String())
}

func (d *Device) AddrDomain() uint16 {
	return d.addr.Domain
}

func (d *Device) AddrBus() uint8 {
	return d.addr.Bus
}

func (d *Device) AddrDevice() uint8 {
	return d.addr.Device
}

func (d *Device) AddrFunction() uint8 {
	return d.addr.Function
}

func (d *Device) String() string {
	return d.addr.String()
}

func (d *Device) Enabled() bool {
	return d.enabled
}

func (d *Device) Parent() *Device {
	return d.parent
}

func (d *Device) Subdevices() []*Device {
	return d.subdevices
}

func (d *Device) HasMultifunctionFeature() bool {
	return d.multifunction
}

func (d *Device) CurrentDriver() string {
	return d.driver
}

func (d *Device) Vendor() uint16 {
	return d.vendorID
}

func (d *Device) VendorHex() string {
	return fmt.Sprintf("0x%04x", d.vendorID)
}

func (d *Device) VendorName() string {
	return d.vendorName
}

func (d *Device) Class() uint32 {
	return d.classID
}

func (d *Device) ClassHex() string {
	return fmt.Sprintf("0x%06x", d.classID)
}

func (d *Device) ClassName() string {
	return d.className
}

func (d *Device) SubclassName() string {
	return d.subclassName
}

func (d *Device) Device() uint16 {
	return d.deviceID
}

func (d *Device) DeviceHex() string {
	return fmt.Sprintf("0x%04x", d.deviceID)
}

func (d *Device) DeviceName() string {
	return d.deviceName
}

func (d *Device) AssignDriver(n string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if n = strings.TrimSpace(n); len(n) == 0 {
		return fmt.Errorf("empty driver name")
	}

	if d.driver == n {
		return nil
	}

	if len(d.driver) > 0 {
		if err := os.WriteFile(filepath.Join(d.FullPath(), "driver/unbind"), []byte(d.String()), 0200); err != nil {
			return fmt.Errorf("failed to unbind: %w", err)
		}
	}

	if err := os.WriteFile(filepath.Join("/sys/bus/pci/drivers", n, "new_id"), []byte(d.VendorHex()+" "+d.DeviceHex()+"\n"), 0200); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("failed to assign driver: is %s loaded?", n)
		}
		return fmt.Errorf("failed to assign driver: %w", err)
	}

	if s, err := filepath.EvalSymlinks(filepath.Join(d.FullPath(), "driver")); err == nil {
		d.driver = filepath.Base(s)
	} else {
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot check the new driver: %w", err)
		}
	}

	if d.driver != n {
		return fmt.Errorf("failed to assign driver: run 'dmesg' for more details")
	}

	return nil
}

func (d *Device) UnbindDriver() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.driver) > 0 {
		if err := os.WriteFile(filepath.Join(d.FullPath(), "driver/unbind"), []byte(d.String()), 0200); err != nil {
			return fmt.Errorf("failed to unbind: %w", err)
		}
	}

	return nil
}

func DeviceList() ([]*Device, error) {
	files, err := os.ReadDir("/sys/bus/pci/devices")
	if err != nil {
		return nil, err
	}

	devices := make([]*Device, 0, len(files))

	results := make(chan *Device)
	defer close(results)

	gr, ctx := errgroup.WithContext(context.Background())

	gr.SetLimit(runtime.NumCPU())

	var syncMap sync.Map

	gr.Go(func() error {
		for _, f := range files {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if f.Type()&os.ModeSymlink == 0 {
				continue
			}

			hexaddr := f.Name()

			if _, ok := syncMap.Load(hexaddr); ok {
				continue
			}

			gr.Go(func() error {
				pcidev, err := lookup(hexaddr)
				if err != nil {
					if errors.Is(err, ErrDeviceNotFound) {
						// The device was probably removed while reading
						return nil
					}
					return err
				}

				results <- pcidev

				syncMap.Store(hexaddr, struct{}{})

				for _, sub := range pcidev.Subdevices() {
					if _, ok := syncMap.Load(sub.String()); !ok {
						results <- sub

						syncMap.Store(sub.String(), struct{}{})
					}
				}

				return nil
			})
		}

		return nil
	})

	go func() {
		for pcidev := range results {
			devices = append(devices, pcidev)
		}
	}()

	if err := gr.Wait(); err != nil {
		return nil, err
	}

	// Sort by PCI address
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].String() < devices[j].String()
	})

	return devices, nil
}

func readHeadBytes(dirname, fname string, count uint) ([]byte, error) {
	file, err := os.Open(filepath.Join(dirname, fname))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf := make([]byte, count)

	n, err := file.Read(buf)
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}

func readString(dirname, fname string) (string, error) {
	s, err := os.ReadFile(filepath.Join(dirname, fname))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(s)), nil
}

func readUint(dirname, fname string, base, bits int) (uint64, error) {
	s, err := readString(dirname, fname)
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(strings.TrimPrefix(s, "0x"), base, bits)
}
