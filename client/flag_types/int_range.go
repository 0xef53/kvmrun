package flag_types

import (
	"fmt"
	"strconv"
	"strings"
)

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

func (t IntRange) Get() interface{} {
	return t
}

func parseIntRange(s string) (min, max int64, err error) {
	fields := strings.SplitN(s, "-", 2)

	switch len(fields) {
	case 1:
		min, err = strconv.ParseInt(fields[0], 10, 0)
		max = min
	case 2:
		min, err = strconv.ParseInt(fields[0], 10, 0)
		if err == nil {
			max, err = strconv.ParseInt(fields[1], 10, 0)
		}
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
