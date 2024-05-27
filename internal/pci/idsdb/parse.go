package idsdb

import (
	"bufio"
	"strings"
)

func parse(db *DB, scanner *bufio.Scanner) error {
	db.Classes = make(map[string]*Class, 20)
	db.Vendors = make(map[string]*Vendor, 200)
	db.Products = make(map[string]*Product, 1000)

	subclasses := make([]*Subclass, 0)
	progIfaces := make([]*ProgrammingInterface, 0)

	vendorProducts := make([]*Product, 0)
	productSubsystems := make([]*Product, 0)

	var curClass *Class
	var curSubclass *Subclass
	var curProgIface *ProgrammingInterface
	var curVendor *Vendor
	var curProduct *Product
	var curSubsystem *Product

	var inClassBlock bool

	for scanner.Scan() {
		line := scanner.Text()

		if len(line) == 0 || strings.HasPrefix(line, "#") {
			// skip comments and blank lines
			continue
		}

		lineBytes := []rune(line)

		// Lines starting with an uppercase "C" indicate a PCI top-level class
		// information block. These lines look like this:
		//
		// C 02  Network controller
		if lineBytes[0] == 'C' {
			if curClass != nil {
				// finalize existing class because we found a new class block
				curClass.Subclasses = subclasses
				subclasses = make([]*Subclass, 0)
			}

			inClassBlock = true

			curClass = &Class{
				ID:         string(lineBytes[2:4]),
				Name:       string(lineBytes[6:]),
				Subclasses: subclasses,
			}

			db.Classes[curClass.ID] = curClass

			continue
		}

		// Lines not beginning with an uppercase "C" or a TAB character
		// indicate a top-level vendor information block. These lines look like
		// this:
		//
		// 0a89  BREA Technologies Inc
		if lineBytes[0] != '\t' {
			if curVendor != nil {
				// finalize existing vendor because we found a new vendor block
				curVendor.Products = vendorProducts
				vendorProducts = make([]*Product, 0)
			}

			inClassBlock = false

			curVendor = &Vendor{
				ID:       string(lineBytes[0:4]),
				Name:     string(lineBytes[6:]),
				Products: vendorProducts,
			}

			db.Vendors[curVendor.ID] = curVendor

			continue
		}

		// Lines beginning with only a single TAB character are *either* a
		// subclass OR are a device information block. If we're in a class
		// block (i.e. the last parsed block header was for a PCI class), then
		// we parse a subclass block. Otherwise, we parse a device information
		// block.
		//
		// A subclass information block looks like this:
		//
		// \t00  Non-VGA unclassified device
		//
		// A device information block looks like this:
		//
		// \t0002  PCI to MCA Bridge
		if len(lineBytes) > 1 && lineBytes[1] != '\t' {
			if inClassBlock {
				if curSubclass != nil {
					// finalize existing subclass because we found a new subclass block
					curSubclass.ProgrammingInterfaces = progIfaces
					progIfaces = make([]*ProgrammingInterface, 0)
				}

				curSubclass = &Subclass{
					ID:                    string(lineBytes[1:3]),
					Name:                  string(lineBytes[5:]),
					ProgrammingInterfaces: progIfaces,
				}

				subclasses = append(subclasses, curSubclass)
			} else {
				if curProduct != nil {
					// finalize existing product because we found a new product block
					curProduct.Subsystems = productSubsystems
					productSubsystems = make([]*Product, 0)
				}

				productID := string(lineBytes[1:5])
				productKey := curVendor.ID + productID

				curProduct = &Product{
					VendorID: curVendor.ID,
					ID:       productID,
					Name:     string(lineBytes[7:]),
				}

				vendorProducts = append(vendorProducts, curProduct)

				db.Products[productKey] = curProduct
			}
		} else {
			// Lines beginning with two TAB characters are *either* a subsystem
			// (subdevice) OR are a programming interface for a PCI device
			// subclass. If we're in a class block (i.e. the last parsed block
			// header was for a PCI class), then we parse a programming
			// interface block, otherwise we parse a subsystem block.
			//
			// A programming interface block looks like this:
			//
			// \t\t00  UHCI
			//
			// A subsystem block looks like this:
			//
			// \t\t0e11 4091  Smart Array 6i
			if inClassBlock {
				curProgIface = &ProgrammingInterface{
					ID:   string(lineBytes[2:4]),
					Name: string(lineBytes[6:]),
				}

				progIfaces = append(progIfaces, curProgIface)
			} else {
				curSubsystem = &Product{
					VendorID: string(lineBytes[2:6]),
					ID:       string(lineBytes[7:11]),
					Name:     string(lineBytes[13:]),
				}

				productSubsystems = append(productSubsystems, curSubsystem)
			}
		}
	}

	return scanner.Err()
}
