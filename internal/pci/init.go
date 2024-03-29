package pci

import (
	"github.com/0xef53/kvmrun/internal/pci/idsdb"
)

var DB *idsdb.DB

func init() {
	db, err := idsdb.Load()
	if err != nil {
		// use a stub DB
		DB = &idsdb.DB{
			Classes:  make(map[string]*idsdb.Class),
			Vendors:  make(map[string]*idsdb.Vendor),
			Products: make(map[string]*idsdb.Product),
		}
	}

	DB = db
}
