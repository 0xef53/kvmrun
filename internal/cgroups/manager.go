//go:build linux
// +build linux

package cgroups

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotMounted = errors.New("controller not mounted")
)

// Manager is a wrapper around the several cgroups to more convenient using.
type Manager struct {
	pid     int
	cgroups map[string]Cgroup
}

// LoadManager tries to find all cgroup in which the specified pid
// is placed and returns a new Manager on success to manage them.
func LoadManager(pid int) (*Manager, error) {
	cgroups, err := GetProcessGroups(pid)
	if err != nil {
		return nil, err
	}

	return &Manager{pid: pid, cgroups: cgroups}, nil
}

// Set applies the parameters specified in the config to the cgroup set.
func (m *Manager) Set(c Config) error {
	for _, g := range m.cgroups {
		if err := g.Set(c); err != nil {
			return err
		}
	}

	return nil
}

// Get returns actual parameters and limits for the cgroup set.
func (m *Manager) Get() (Config, error) {
	c := newConfig()

	for _, g := range m.cgroups {
		if err := g.Get(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// GetStats returns usage statistics for the cgroup set.
func (m *Manager) Stat() (Stat, error) {
	stat := newStat()

	for _, g := range m.cgroups {
		if err := g.Stat(stat); err != nil {
			return nil, err
		}
	}

	return stat, nil
}

func (m *Manager) GetNetClassID() (int64, error) {
	g, ok := m.cgroups["net_cls"]
	if !ok {
		return 0, fmt.Errorf("%w: net_cls", ErrNotMounted)
	}

	cfg := newConfig()

	if err := g.Get(cfg); err != nil {
		return 0, err
	}

	if value, ok := cfg["net_cls.classid"]; ok {
		if value != nil && value.Len() == 1 {
			return value.First().AsInt64()
		}
	}

	return 0, nil
}

func (m *Manager) SetNetClassID(classID int64) error {
	g, ok := m.cgroups["net_cls"]
	if !ok {
		return fmt.Errorf("%w: net_cls", ErrNotMounted)
	}

	cfg := newConfig()

	cfg["net_cls.classid"] = NewValue(classID)

	return g.Set(cfg)
}

func (m *Manager) GetCpuQuota() (int64, error) {
	g, ok := m.cgroups["cpu"]
	if !ok {
		return 0, fmt.Errorf("%w: cpu", ErrNotMounted)
	}

	cfg := newConfig()

	if err := g.Get(cfg); err != nil {
		return 0, err
	}

	var timeQuota, period int64
	var err error

	if g.Version() == 1 {
		timeQuota, err = func() (int64, error) {
			if value, ok := cfg["cpu.cfs_quota_us"]; ok {
				if value != nil && value.Len() == 1 {
					return value.First().AsInt64()
				}
			}
			return 0, ErrCfsNotSupported
		}()
		if err != nil {
			return 0, err
		}

		if timeQuota == -1 {
			// ok, no limit set
			timeQuota = 0
		}

		period, err = func() (int64, error) {
			if value, ok := cfg["cpu.cfs_period_us"]; ok {
				if value != nil && value.Len() == 1 {
					return value.First().AsInt64()
				}
			}
			return 0, ErrCfsNotSupported
		}()
		if err != nil {
			return 0, err
		}
	} else {
		timeQuota, period, err = func() (int64, int64, error) {
			if value, ok := cfg["cpu.max"]; ok {
				if value != nil && value.Len() == 2 {
					quota, err := value.First().AsInt64()
					if err != nil {
						// check, maybe there is a string "max"
						if s, err := value.First().AsString(); err == nil {
							if strings.ToLower(s) == "max" {
								// ok, no limit set
								return 0, 0, nil
							} else {
								// got too weird value: string, but not "max"
								return 0, 0, fmt.Errorf("invalid value of cpu.max, time_quota: %s", s)
							}
						} else {
							return 0, 0, fmt.Errorf("invalid value of cpu.max")
						}
					}

					period, err := value.Second().AsInt64()
					if err != nil {
						return 0, 0, err
					}

					return quota, period, nil
				}
			}

			return 0, 0, ErrCfsNotSupported
		}()
		if err != nil {
			return 0, err
		}
	}

	if timeQuota == 0 || period == 0 {
		// ok, no limit set
		return 0, nil
	}

	return timeQuota * 100 / period, nil
}

func (m *Manager) SetCpuQuota(quota int64) error {
	g, ok := m.cgroups["cpu"]
	if !ok {
		return fmt.Errorf("%w: cpu", ErrNotMounted)
	}

	curCfg := newConfig()
	newCfg := newConfig()

	if err := g.Get(curCfg); err != nil {
		return err
	}

	if g.Version() == 1 {
		if quota == 0 {
			newCfg["cpu.cfs_quota_us"] = NewValue(-1)
		} else {
			period, err := func() (int64, error) {
				if value, ok := curCfg["cpu.cfs_period_us"]; ok {
					if value != nil && value.Len() == 1 {
						return value.First().AsInt64()
					}
				}
				return 0, ErrCfsNotSupported
			}()
			if err != nil {
				return err
			}

			newCfg["cpu.cfs_quota_us"] = NewValue((period * quota) / 100)
		}
	} else {
		if quota == 0 {
			newCfg["cpu.max"] = NewValue("max")
		} else {
			period, err := func() (int64, error) {
				if value, ok := curCfg["cpu.max"]; ok {
					if value != nil && value.Len() == 2 {
						return value.Second().AsInt64()
					}
				}
				return 0, ErrCfsNotSupported
			}()
			if err != nil {
				return err
			}

			newCfg["cpu.max"] = NewValue((period * quota) / 100)
		}
	}

	return g.Set(newCfg)
}
