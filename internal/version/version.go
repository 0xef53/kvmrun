package version

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrInvalidValue     = errors.New("invalid value")
	ErrInvalidValueType = errors.New("invalid value type")
)

type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Micro int `json:"micro"`
}

func Parse(a interface{}) (*Version, error) {
	return parse(a)
}

func MustParse(a interface{}) *Version {
	v, err := parse(a)
	if err != nil {
		return &Version{}
	}

	return v
}

func parse(a interface{}) (*Version, error) {
	switch v := a.(type) {
	case string:
		return parseString(v)
	case int:
		return parseInt(v)
	}

	return nil, fmt.Errorf("%w: %T", ErrInvalidValueType, a)
}

func parseString(s string) (*Version, error) {
	parts := strings.Split(strings.TrimSpace(s), ".")

	if len(parts) > 3 {
		return nil, fmt.Errorf("%w: %s", ErrInvalidValue, s)
	}

	vv := make([]int, len(parts))

	for idx, part := range parts {
		if v, err := strconv.Atoi(part); err == nil {
			vv[idx] = v
		} else {
			return nil, err
		}
	}

	for i := len(parts); i <= 3; i++ {
		vv = append(vv, 0)
	}

	return &Version{
		Major: vv[0],
		Minor: vv[1],
		Micro: vv[2],
	}, nil
}

func parseInt(v int) (*Version, error) {
	return &Version{
		Major: v / 10000,
		Minor: (v % 10000) / 100,
		Micro: (v % 10000) % 100,
	}, nil
}

func (v Version) Int() int {
	return v.Major*10000 + v.Minor*100 + v.Micro
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Micro)
}
