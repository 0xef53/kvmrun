//+build linux

package cgroup

import (
	"errors"
)

var (
	ErrCgroupRemoved = errors.New("Unable to continue. Control group already removed")
)

// Manager is a wrapper around the several cgroups to more convenient using.
type Manager struct {
	pid       int
	cgroups   map[string]Cgroup
	isRemoved bool
}

// NewManager creates a new cgroup for the specified pid in each
// of the included subsystems (determined using the /proc/cgroups file).
//
// The function returns a new Manager object to manage the cgroup set.
func NewManager(pid int, subpath string, subsystems ...string) (*Manager, error) {
	cgroupList, err := GetEnabledSubsystems()
	if err != nil {
		return nil, err
	}

	cgroups := make(map[string]Cgroup)

	for _, s := range subsystems {
		if _, ok := cgroupList[s]; !ok {
			continue
		}
		switch s {
		case "cpu":
			g, err := NewCpuGroup(subpath, pid)
			if err != nil {
				return nil, err
			}
			cgroups[s] = g
		default:
			return nil, NewUnsupportedError(s)
		}
	}

	return &Manager{pid: pid, cgroups: cgroups}, nil
}

// LoadManager tries to find all cgroup in which the specified pid
// is placed and returns a new Manager on success to manage them.
func LoadManager(pid int) (*Manager, error) {
	cgroupList, err := GetProcessCgroups(pid)
	if err != nil {
		return nil, err
	}

	cgroups := make(map[string]Cgroup)

	for s, _ := range cgroupList {
		switch g, err := LookupCgroupByPid(pid, s); {
		case err == nil:
			cgroups[s] = g
		case IsUnsupportedError(err):
			continue
		default:
			return nil, err
		}
	}

	return &Manager{pid: pid, cgroups: cgroups}, nil
}

// Set applies the parameters specified in the config to the cgroup set.
func (m *Manager) Set(c *Config) error {
	if m.isRemoved {
		return ErrCgroupRemoved
	}

	for _, g := range m.cgroups {
		if err := g.Set(c); err != nil {
			return err
		}
	}

	return nil
}

// Get returns actual parameters and limits for the cgroup set.
func (m *Manager) Get(c *Config) error {
	if m.isRemoved {
		return ErrCgroupRemoved
	}

	for _, g := range m.cgroups {
		if err := g.Get(c); err != nil {
			return err
		}
	}

	return nil
}

// GetStats returns usage statistics for the cgroup set.
func (m *Manager) GetStats(stats *Stats) error {
	if m.isRemoved {
		return ErrCgroupRemoved
	}

	for _, g := range m.cgroups {
		if err := g.GetStats(stats); err != nil {
			return err
		}
	}

	return nil
}

// Destroy destroys the cgroup set.
func (m *Manager) Destroy() error {
	m.isRemoved = true

	for _, g := range m.cgroups {
		if err := DestroyCgroup(g.GetPath()); err != nil {
			return err
		}
	}

	return nil
}
