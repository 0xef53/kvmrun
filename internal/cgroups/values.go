package cgroups

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrTypeAssertion = errors.New("type assertion error")
)

type ValueUnit struct {
	value interface{}
}

func (u *ValueUnit) AsString() (s string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: %w", ErrTypeAssertion, err)
		}
	}()

	switch v := u.value.(type) {
	case string:
		return v, nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), nil
	}

	return "", fmt.Errorf("unsupported base type: %T", u.value)
}

func (u *ValueUnit) AsInt64() (v int64, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: %w", ErrTypeAssertion, err)
		}
	}()

	switch v := u.value.(type) {
	case string:
		return strconv.ParseInt(v, 10, 64)
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return int64(v), nil
	}

	return 0, fmt.Errorf("unsupported base type: %T", u.value)
}

func (u *ValueUnit) MarshalJSON() ([]byte, error) {
	s, err := u.AsString()
	if err != nil {
		s = fmt.Sprintf("<invalid value: %s>", err)
	}

	return json.Marshal(s)
}

type Value struct {
	units []*ValueUnit
}

func NewValue(a ...interface{}) *Value {
	units := make([]*ValueUnit, 0, len(a))

	for _, u := range a {
		units = append(units, &ValueUnit{value: u})
	}

	return &Value{units: units}
}

func ParseValue(a string) *Value {
	strs := strings.Fields(a)

	units := make([]*ValueUnit, 0, len(strs))

	for _, s := range strs {
		units = append(units, &ValueUnit{value: s})
	}

	return &Value{units: units}
}

func (v *Value) AsString() (s string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot be represented as string: %w", err)
		}
	}()

	strs := make([]string, len(v.units))

	for idx := range v.units {
		if s, err := v.units[idx].AsString(); err == nil {
			strs[idx] = s
		} else {
			return "", err
		}
	}

	return strings.Join(strs, " "), nil
}

func (v *Value) First() *ValueUnit {
	if len(v.units) >= 1 {
		return v.units[0]
	}
	return nil
}

func (v *Value) Second() *ValueUnit {
	if len(v.units) >= 2 {
		return v.units[1]
	}
	return nil
}

func (v *Value) Len() int {
	return len(v.units)
}

func (v *Value) MarshalJSON() ([]byte, error) {
	s, err := v.AsString()
	if err != nil {
		s = "<invalid value>"
	}

	return json.Marshal(s)
}
