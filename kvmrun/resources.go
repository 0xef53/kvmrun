package kvmrun

import (
	"fmt"
	"strings"
)

// Memory represents the guest memory configuration structure.
type Memory struct {
	Actual int `json:"actual"`
	Total  int `json:"total"`
}

func (m *Memory) Validate(_ bool) error {
	if m.Actual < 0 || m.Total < 0 {
		return fmt.Errorf("invalid memory value: cannot be less than 0")
	}

	if m.Actual > m.Total {
		return fmt.Errorf("invalid memory value: total cannot be less than actual")
	}

	return nil
}

func (m *Memory) Copy() *Memory {
	return &Memory{
		Actual: m.Actual,
		Total:  m.Total,
	}
}

func (m *Memory) SetActual(value int) error {
	if value < 1 {
		return fmt.Errorf("invalid memory size: cannot be less than 1")
	}

	if value > m.Total {
		return fmt.Errorf("invalid actual memory: cannot be large than total memory (%d)", m.Total)
	}

	m.Actual = value

	return nil
}

func (m *Memory) SetTotal(value int) error {
	if value < 1 {
		return fmt.Errorf("invalid memory size: cannot be less than 1")
	}

	if value < m.Actual {
		return fmt.Errorf("invalid total memory: cannot be less than actual memory (%d)", m.Actual)
	}

	m.Total = value

	return nil
}

// VirtCPU represents the guest virtual CPU configuration structure.
type VirtCPU struct {
	Actual  int    `json:"actual"`
	Total   int    `json:"total"`
	Sockets int    `json:"sockets,omitempty"`
	Quota   int    `json:"quota,omitempty"`
	Model   string `json:"model,omitempty"`
}

func (v *VirtCPU) Validate(_ bool) error {
	if v.Actual < 0 || v.Total < 0 {
		return fmt.Errorf("invalid vCPU value: cannot be less than 0")
	}

	if v.Actual > v.Total {
		return fmt.Errorf("invalid vCPU value: total cannot be less than actual")
	}

	if v.Sockets < 0 {
		return fmt.Errorf("invalid vCPU socket count value: cannot be less than 0")
	}

	if v.Quota < 0 {
		return fmt.Errorf("invalid vCPU quota value: cannot be less than 0")
	}

	v.Model = strings.TrimSpace(v.Model)

	return nil
}

func (v *VirtCPU) Copy() *VirtCPU {
	return &VirtCPU{
		Actual:  v.Actual,
		Total:   v.Total,
		Sockets: v.Sockets,
		Quota:   v.Quota,
		Model:   v.Model,
	}
}

func (v *VirtCPU) SetActual(value int) error {
	if value < 1 {
		return fmt.Errorf("invalid cpu count: cannot be less than 1")
	}

	if value > v.Total {
		return fmt.Errorf("invalid actual cpu: cannot be large than total cpu (%d)", v.Total)
	}

	v.Actual = value

	return nil
}

func (v *VirtCPU) SetTotal(value int) error {
	if value < 1 {
		return fmt.Errorf("invalid cpu count: cannot be less than 1")
	}

	if value < v.Actual {
		return fmt.Errorf("invalid total cpu: cannot be less than actual cpu (%d)", v.Actual)
	}

	v.Total = value

	return nil
}

func (v *VirtCPU) SetSockets(value int) error {
	if value < 0 {
		return fmt.Errorf("invalid number of processor sockets: cannot be less than 0")
	}

	if v.Total%value != 0 {
		return fmt.Errorf("invalid number of processor sockets: total cpu count must be multiple of %d", value)
	}

	v.Sockets = value

	return nil
}

func (v *VirtCPU) SetModel(value string) error {
	v.Model = strings.TrimSpace(value)

	return nil
}

func (v *VirtCPU) SetQuota(value int) error {
	if value < 5 {
		return fmt.Errorf("invalid number of cpu quota: cannot be less than 5")
	}

	v.Quota = value

	return nil
}
