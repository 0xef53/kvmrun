package types

import (
	"fmt"
	"strconv"
	"strings"
)

func parseIntRange(s string) (min, max int64, err error) {
	fields := strings.SplitN(s, "-", 2)

	switch len(fields) {
	case 1:
		min, err = strconv.ParseInt(fields[0], 10, 0)
		max = min
	case 2:
		min, err = strconv.ParseInt(fields[0], 10, 0)
		max, err = strconv.ParseInt(fields[1], 10, 0)
	}

	errInvalidRange := fmt.Errorf("incorrect range: %s", s)

	if err != nil {
		return 0, 0, errInvalidRange
	}

	if max < min {
		return 0, 0, errInvalidRange
	}

	return min, max, nil
}

type IntRange struct {
	Min int64
	Max int64
}

func (t *IntRange) Set(value string) (err error) {
	t.Min, t.Max, err = parseIntRange(value)
	if err != nil {
		return err
	}

	if t.Min == 0 {
		return fmt.Errorf("min value cannot be equal to 0")
	}

	if t.Min > t.Max {
		return fmt.Errorf("max value cannot be less than min")
	}

	return nil
}

func (t IntRange) String() string {
	return fmt.Sprintf("%d-%d", t.Min, t.Max)
}

func (t IntRange) Value() IntRange {
	return t
}

type StringMap struct {
	m   map[string]string
	sep string
}

func NewStringMap(sep string) *StringMap {
	return &StringMap{m: make(map[string]string), sep: sep}
}

func (t *StringMap) Set(value string) error {
	parts := strings.Split(value, t.sep)

	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		return fmt.Errorf("a map value should be specified as 'valueA%svalueB'", t.sep)
	}

	if _, ok := t.m[parts[0]]; ok {
		return fmt.Errorf("it's a duplicate: %s", value)
	}

	t.m[parts[0]] = parts[1]

	return nil
}

func (t StringMap) String() string {
	lines := []string{}
	for k, v := range t.m {
		lines = append(lines, "\""+k+t.sep+v+"\"")
	}

	return "[ " + strings.Join(lines, ", ") + " ]"
}

func (t StringMap) Value() map[string]string {
	return t.m
}

type StringBool struct {
	value bool
}

func NewStringBool() *StringBool {
	return &StringBool{value: false}
}

func (t *StringBool) Set(s string) error {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "on", "yes", "true":
		t.value = true
	case "0", "off", "no", "false":
		t.value = false
	default:
		return fmt.Errorf("incorrect value: %s", s)
	}

	return nil
}

func (t StringBool) String() string {
	if t.value {
		return "on"
	}

	return "off"
}

func (t StringBool) Value() bool {
	return t.value
}
