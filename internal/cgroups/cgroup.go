//go:build linux
// +build linux

// Package cgroups provides primitives to work with Linux Control Groups
// via pseudo file system /sys/fs/cgroup.
//
// https://docs.kernel.org/admin-guide/cgroup-v1/cgroups.html
// https://docs.kernel.org/admin-guide/cgroup-v2.html

package cgroups

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrUnknownParameter = errors.New("unknown cgroup parameter")
)

// Cgroup is an uniform interface for the cgroups.
type Cgroup interface {
	// Sets the cgroup parameters represented by *Config variable
	Set(Config) error

	// Gets the actual cgroup parameters and stores them to the pointer of *Config
	Get(Config) error

	// Gets the cgroup stat information and store its to the pointer of *Stats
	Stat(Stat) error

	// Returns full path of the cgroup relative to the filesystem root
	Path() string

	Version() uint16
}

// Config specifies parameters for the various controllers.
type Config map[string]*Value

func newConfig() Config {
	return make(map[string]*Value)
}

// Stats contains metrics and limits from each of the cgroup subsystems.
type Stat map[string]*ValueUnit

func newStat() Stat {
	return make(map[string]*ValueUnit)
}

// GetProcessCgroups returns a map with the all cgroup subsystems
// to which the specified pid belongs.
func GetProcessGroups(pid int) (map[string]Cgroup, error) {
	mpoints, err := func() (map[string]string, error) {
		m := make(map[string]string)

		fd, err := os.Open("/proc/self/mountinfo")
		if err != nil {
			return nil, err
		}
		defer fd.Close()

		scanner := bufio.NewScanner(fd)

		for scanner.Scan() {
			ff := strings.Split(scanner.Text(), " ")

			if len(ff) < 10 {
				return nil, fmt.Errorf("cannot parsing %s: unknown format", fd.Name())
			}

			switch ff[len(ff)-3] {
			case "cgroup2":
				m["v2unified"] = ff[4]
			case "cgroup":
				for _, part := range strings.Split(ff[len(ff)-1], ",") {
					if part != "rw" {
						if _, ok := m[part]; !ok {
							m[part] = ff[4]
						}
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}

		return m, nil
	}()
	if err != nil {
		return nil, err
	}

	var v2unifiedPath string
	var cgroups = make(map[string]Cgroup)

	group := func(ctrl, fullpath string, version uint16) Cgroup {
		switch ctrl {
		case "cpu":
			return &Group_CPU{path: fullpath, version: version}
		case "net_cls":
			return &Group_NETCLS{path: fullpath, version: version}
		}
		return nil
	}

	fd, err := os.Open(filepath.Join("/proc", fmt.Sprintf("%d", pid), "cgroup"))
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)

	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("cannot parsing %s: unknown format", fd.Name())
		}

		// For the cgroups v2 hierarchy, this field contains the value 0 (see cgroups(7))
		if parts[0] == "0" {
			v2unifiedPath = parts[2]
		} else {
			for _, ctrl := range strings.Split(parts[1], ",") {
				if mp, ok := mpoints[ctrl]; ok {
					if g := group(ctrl, filepath.Join(mp, parts[2]), 1); g != nil {
						cgroups[ctrl] = g
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if mp, ok := mpoints["v2unified"]; ok && len(v2unifiedPath) != 0 {
		b, err := os.ReadFile(filepath.Join(mp, "cgroup.subtree_control"))
		if err != nil {
			return nil, err
		}

		for _, ctrl := range strings.Fields(string(b)) {
			if _, ok := cgroups[ctrl]; !ok {
				if g := group(ctrl, filepath.Join(mp, v2unifiedPath), 2); g != nil {
					cgroups[ctrl] = g
				}
			}
		}
	}

	if len(cgroups) == 0 {
		return nil, nil
	}

	return cgroups, nil
}

func writeValue(dir, fname string, v *Value) error {
	data, err := v.AsString()
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, fname), []byte(data), 0700)
}

func readValue(dir, fname string) (*Value, error) {
	b, err := os.ReadFile(filepath.Join(dir, fname))
	if err != nil {
		return nil, err
	}

	return ParseValue(string(b)), nil
}
