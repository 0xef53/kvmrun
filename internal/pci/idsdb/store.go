package idsdb

import (
	"io/fs"
	"os"
	"path/filepath"
)

type Store interface {
	Open(string) (fs.File, error)
}

type Dir string

func (d Dir) Open(name string) (fs.File, error) {
	return os.Open(filepath.Join(string(d), name))
}
