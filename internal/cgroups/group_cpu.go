//go:build linux
// +build linux

package cgroups

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	ErrCfsNotSupported = errors.New("make sure that CONFIG_CFS_BANDWIDTH option is enabled in your kernel")
	ErrRtNotSupported  = errors.New("make sure that CONFIG_RT_GROUP_SCHED option is enabled in your kernel")
)

// Group_CPU is an implementation of the common Cgroup interface.
type Group_CPU struct {
	path    string
	version uint16
}

func (g *Group_CPU) Path() string {
	return g.path
}

func (g *Group_CPU) Version() uint16 {
	return g.version
}

func (g *Group_CPU) Set(c Config) error {
	var params map[string]struct{}

	if g.version == 1 {
		params = map[string]struct{}{
			"cpu.cfs_period_us": struct{}{},
			"cpu.cfs_quota_us":  struct{}{},
			"cpu.rt_period_us":  struct{}{},
			"cpu.rt_runtime_us": struct{}{},
		}
	} else {
		params = map[string]struct{}{
			"cpu.max": struct{}{},
		}
	}

	for param, v := range c {
		var err error

		if _, ok := params[param]; ok {
			err = writeValue(g.path, param, v)
			if os.IsNotExist(err) {
				switch param {
				case "cpu.cfs_period_us", "cpu.cfs_quota_us":
					err = ErrCfsNotSupported
				case "cpu.rt_period_us", "cpu.rt_runtime_us":
					err = ErrRtNotSupported
				}
			}
		} else {
			err = fmt.Errorf("%w: %s", ErrUnknownParameter, param)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (g *Group_CPU) Get(c Config) error {
	if c == nil {
		c = newConfig()
	}

	var params []string

	if g.version == 1 {
		params = []string{
			"cpu.cfs_period_us",
			"cpu.cfs_quota_us",
			"cpu.rt_period_us",
			"cpu.rt_runtime_us",
		}
	} else {
		params = []string{
			"cpu.max",
		}
	}

	for _, param := range params {
		if v, err := readValue(g.path, param); err == nil {
			c[param] = v
		} else {
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	return nil
}

func (g *Group_CPU) Stat(stat Stat) error {
	if stat == nil {
		stat = newStat()
	}

	fd, err := os.Open(filepath.Join(g.path, "cpu.stat"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)

	for scanner.Scan() {
		value := ParseValue(scanner.Text())
		if value.Len() == 2 {
			param, err := value.First().AsString()
			if err != nil {
				return err
			}
			stat[param] = value.Second()
		}
	}

	return scanner.Err()
}
