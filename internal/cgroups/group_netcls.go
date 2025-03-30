//go:build linux
// +build linux

package cgroups

import (
	"fmt"
)

// NetClsGroup is an implementation of the common Cgroup interface.
type Group_NETCLS struct {
	path    string
	version uint16
}

// GetPath return a full path to the directory of the current cgroup.
func (g *Group_NETCLS) Path() string {
	return g.path
}

func (g *Group_NETCLS) Version() uint16 {
	return g.version
}

// Set applies the parameters specified in the config to the current cgroup.
func (g *Group_NETCLS) Set(c Config) error {
	if g.version == 2 {
		return fmt.Errorf("net_cls controller is not supported in cgroups version 2")
	}

	var err error

	for param, v := range c {
		switch param {
		case "net_cls.classid":
			err = writeValue(g.path, param, v)
		default:
			err = fmt.Errorf("%w: %s", ErrUnknownParameter)
		}

		return err
	}

	return nil
}

// Get returns actual parameters and limits for the current cgroup.
func (g *Group_NETCLS) Get(c Config) error {
	if g.version == 2 {
		return fmt.Errorf("net_cls controller is not supported in cgroups version 2")
	}

	if c == nil {
		c = newConfig()
	}

	if v, err := readValue(g.path, "net_cls.classid"); err == nil {
		c["net_cls.classid"] = v
	} else {
		return err
	}

	return nil
}

// GetStats returns usage statistics for the current cgroup.
func (g *Group_NETCLS) Stat(stat Stat) error {
	// The NET_CLS controller does not provide any statistics
	return nil
}
