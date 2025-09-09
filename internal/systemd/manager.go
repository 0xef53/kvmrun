package systemd

import (
	"encoding/json"
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

type UnitStatus struct {
	Name        string // The primary unit name as string
	LoadState   string // The load state (i.e. whether the unit file has been loaded successfully)
	ActiveState string // The active state (i.e. whether the unit is currently started or not)
	SubState    string // The sub state (a more fine-grained version of the active state that is specific to the unit type, which the active state is not)
}

type Manager struct {
	conn *dbus.Conn
}

func NewManager() (*Manager, error) {
	c, err := dbus.New()
	if err != nil {
		return nil, err
	}

	return &Manager{conn: c}, nil
}

func (m *Manager) Close() {
	m.conn.Close()
}

func (m *Manager) Enable(unitname string) error {
	if _, _, err := m.conn.EnableUnitFiles([]string{unitname}, false, true); err != nil {
		return err
	}

	return nil
}

func (m *Manager) Disable(unitname string) error {
	if _, err := m.conn.DisableUnitFiles([]string{unitname}, false); err != nil {
		return err
	}

	m.conn.ResetFailedUnit(unitname)

	return nil
}

func (m *Manager) Kill(unitname string, signal syscall.Signal) {
	m.conn.KillUnit(unitname, int32(signal))
}

func (m *Manager) KillBySIGKILL(unitname string) {
	m.Kill(unitname, syscall.SIGKILL)
}

func (m *Manager) KillBySIGTERM(unitname string) {
	m.Kill(unitname, syscall.SIGTERM)
}

func (m *Manager) Start(unitname string, ch chan<- string) error {
	if _, err := m.conn.StartUnit(unitname, "replace", ch); err != nil {
		return err
	}

	return nil
}

func (m *Manager) StartAndTest(unitname string, interval time.Duration, ch chan<- string) error {
	if _, err := m.conn.StartUnit(unitname, "replace", ch); err != nil {
		return err
	}

	var lastStateChangeTime time.Time

	waitActivation := func() error {
		done := make(chan struct{})
		defer close(done)

		timeout := make(chan struct{})

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		go func() {
			defer close(timeout)

			select {
			case <-done:
			case <-time.After(300 * time.Second):
			}
		}()

		for {
			select {
			case <-timeout:
				return fmt.Errorf("activation timeout: not ready after %s", time.Duration(300*time.Second))
			case <-ticker.C:
			}

			unit, err := m.GetUnitProperties(unitname)
			if err != nil {
				return err
			}

			switch unit.ActiveState {
			case "active":
				if unit.SubState == "running" {
					lastStateChangeTime = unit.StateChangeTime

					return nil
				}
			case "failed":
				for _, c := range unit.ExecStartPre {
					if c.ExecCode != CodeUndefined && c.ExecCode != CodeExited {
						return fmt.Errorf("pre-start command failed (exit code = %d): %s", c.ExecStatus, strings.Join(c.Arguments, " "))
					}
				}
				for _, c := range unit.ExecStart {
					if c.ExecCode != CodeUndefined {
						return fmt.Errorf("launcher failed (exit code = %d)", c.ExecStatus)
					}
				}
			}
		}
	}

	test := func() error {
		done := make(chan struct{})
		defer close(done)

		timeout := make(chan struct{})

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		go func() {
			defer close(timeout)

			select {
			case <-done:
			case <-time.After(interval):
			}
		}()

		for {
			select {
			case <-timeout:
				// It's OK, the main process is fine
				return nil
			case <-ticker.C:
			}

			unit, err := m.GetUnitProperties(unitname)
			if err != nil {
				return err
			}
			if lastStateChangeTime != unit.StateChangeTime {
				return fmt.Errorf("the main process is crashed")
			}
		}
	}

	if err := waitActivation(); err != nil {
		return err
	}

	return test()
}

func (m *Manager) Stop(unitname string, ch chan<- string) error {
	if _, err := m.conn.StopUnit(unitname, "replace", ch); err != nil {
		return err
	}

	return nil
}

func (m *Manager) StopAndWait(unitname string, interval time.Duration, ch chan<- string) error {
	if _, err := m.conn.StopUnit(unitname, "replace", ch); err != nil {
		return err
	}

	allExecFinished := func(array []ExecCommandState) bool {
		for _, c := range array {
			if c.ExecCode == CodeUndefined {
				return false
			}
		}
		return true
	}

	waitDeactivation := func() error {
		done := make(chan struct{})
		defer close(done)

		timeout := make(chan struct{})

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		go func() {
			defer close(timeout)

			select {
			case <-done:
			case <-time.After(interval):
			}
		}()

	LOOP:
		for {
			select {
			case <-timeout:
				return fmt.Errorf("deactivation timeout: not ready after %s", interval)
			case <-ticker.C:
			}

			unit, err := m.GetUnitProperties(unitname)
			if err != nil {
				return err
			}

			switch unit.ActiveState {
			case "inactive":
				break LOOP
			case "failed":
				// Check the ExecStopPost state despite of the unit status
				if allExecFinished(unit.ExecStopPost) {
					// ExecStopPost command ended one way or another
					break LOOP
				}
			}
		}

		return nil
	}

	return waitDeactivation()
}

func (m *Manager) Restart(unitname string, ch chan<- string) error {
	if _, err := m.conn.RestartUnit(unitname, "replace", ch); err != nil {
		return err
	}

	return nil
}

func (m *Manager) GetUnit(unitname string) (*UnitStatus, error) {
	raw, err := m.conn.ListUnitsByNames([]string{unitname})
	if err != nil {
		return nil, err
	}

	if len(raw) != 1 {
		return nil, fmt.Errorf("unit not found: %s", unitname)
	}

	return &UnitStatus{
		Name:        raw[0].Name,
		LoadState:   raw[0].LoadState,
		ActiveState: raw[0].ActiveState,
		SubState:    raw[0].SubState,
	}, nil
}

func (m *Manager) GetAllUnits(pattern string) ([]*UnitStatus, error) {
	raw, err := m.conn.ListUnitsByPatterns(nil, []string{pattern})
	if err != nil {
		return nil, err
	}

	units := make([]*UnitStatus, 0, len(raw))

	for _, u := range raw {
		units = append(units, &UnitStatus{
			Name:        u.Name,
			LoadState:   u.LoadState,
			ActiveState: u.ActiveState,
			SubState:    u.SubState,
		})
	}

	return units, nil
}

func (m *Manager) GetUnitProperties(unitname string) (*UnitProperties, error) {
	raw, err := m.conn.GetAllProperties(unitname)
	if err != nil {
		return nil, err
	}

	for _, key := range []string{"ActiveEnterTimestamp", "ActiveExitTimestamp", "StateChangeTimestamp"} {
		if _, ok := raw[key]; ok {
			if v, ok := raw[key].(int64); ok {
				raw[strings.TrimSuffix(key, "stamp")] = time.Unix(0, v*int64(time.Microsecond)).String()
			}
		}
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	p := UnitProperties{Name: unitname}

	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}

	return &p, nil
}
