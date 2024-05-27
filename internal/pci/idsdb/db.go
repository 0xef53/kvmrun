package idsdb

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProgrammingInterface is the PCI programming interface for a class of PCI devices
type ProgrammingInterface struct {
	// hex-encoded PCI_ID of the programming interface
	ID string `json:"id"`
	// common string name for the programming interface
	Name string `json:"name"`
}

// Subclass is a subdivision of a PCI class
type Subclass struct {
	// hex-encoded PCI_ID for the device subclass
	ID string `json:"id"`
	// common string name for the subclass
	Name string `json:"name"`
	// any programming interfaces this subclass might have
	ProgrammingInterfaces []*ProgrammingInterface `json:"programming_interfaces"`
}

// Class is the PCI class
type Class struct {
	// hex-encoded PCI_ID for the device class
	ID string `json:"id"`
	// common string name for the class
	Name string `json:"name"`
	// any subclasses belonging to this class
	Subclasses []*Subclass `json:"subclasses"`
}

// Product provides information about a PCI device model
type Product struct {
	// vendor ID for the product
	VendorID string `json:"vendor_id"`
	// hex-encoded PCI_ID for the product/model
	ID string `json:"id"`
	// common string name of the vendor
	Name string `json:"name"`
	// "subdevices" or "subsystems" for the product
	Subsystems []*Product `json:"subsystems"`
}

// Vendor provides information about a device vendor
type Vendor struct {
	// hex-encoded PCI_ID for the vendor
	ID string `json:"id"`
	// common string name of the vendor
	Name string `json:"name"`
	// all top-level devices for the vendor
	Products []*Product `json:"products"`
}

type DB struct {
	// hash of class ID -> class information
	Classes map[string]*Class `json:"classes"`
	// hash of vendor ID -> vendor information
	Vendors map[string]*Vendor `json:"vendors"`
	// hash of vendor ID + product/device ID -> product information
	Products map[string]*Product `json:"products"`
}

func (db *DB) FindClass(hexnum string) (*Class, bool) {
	hexnum = strings.TrimPrefix(hexnum, "0x")

	if len(hexnum) != 6 || db.Classes == nil {
		return nil, false
	}

	if c, ok := db.Classes[hexnum[0:2]]; ok {
		class := Class{ID: c.ID, Name: c.Name}

		for _, s := range c.Subclasses {
			if s.ID == hexnum[2:4] {
				subclass := Subclass{ID: s.ID, Name: s.Name}

				for _, iface := range s.ProgrammingInterfaces {
					if iface.ID == hexnum[4:6] {
						subclass.ProgrammingInterfaces = append(subclass.ProgrammingInterfaces, &ProgrammingInterface{
							ID:   iface.ID,
							Name: iface.Name,
						})
					}
				}

				class.Subclasses = append(class.Subclasses, &subclass)
			}
		}

		return &class, true
	}

	return nil, false
}

func (db *DB) FindVendor(hexvendor string) (*Vendor, bool) {
	hexvendor = strings.TrimPrefix(hexvendor, "0x")

	if len(hexvendor) != 4 || db.Vendors == nil {
		return nil, false
	}

	if v, ok := db.Vendors[hexvendor]; ok {
		return &Vendor{ID: v.ID, Name: v.Name}, true
	}

	return nil, false
}

func (db *DB) FindProduct(hexvendor, hexdevice string) (*Product, bool) {
	hexvendor = strings.TrimPrefix(hexvendor, "0x")
	hexdevice = strings.TrimPrefix(hexdevice, "0x")

	if len(hexvendor) != 4 || len(hexdevice) != 4 || db.Products == nil {
		return nil, false
	}

	if p, ok := db.Products[hexvendor+hexdevice]; ok {
		return &Product{
			VendorID: p.VendorID,
			ID:       p.ID,
			Name:     p.Name,
		}, true
	}

	return nil, false
}

func Load() (*DB, error) {
	var store Store

	if os.Getenv("USE_EMBEDDED_PCIDB") == "1" {
		store = embedStore
	} else {
		if v, err := lookFor("pci.ids"); err == nil {
			store = Dir(v)
		} else {
			store = embedStore
		}
	}

	db := new(DB)

	fd, err := store.Open("pci.ids")
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	if err := parse(db, bufio.NewScanner(fd)); err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	return db, nil
}

func lookFor(fname string) (string, error) {
	possibleDirs := []string{
		".",
		"/usr/share/hwdata",
		"/usr/share/misc",
	}

	var err error

	for _, d := range possibleDirs {
		switch _, err = os.Stat(filepath.Join(d, fname)); {
		case err == nil:
			return d, nil
		case os.IsNotExist(err):
			continue
		default:
			return "", err
		}
	}

	return "", err
}
