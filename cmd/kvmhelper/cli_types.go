package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

func ParseIntRange(s string) (min, max int64, err error) {
	data := strings.SplitN(s, "-", 2)

	switch len(data) {
	case 1:
		min, err = strconv.ParseInt(data[0], 10, 0)
		max = min
	case 2:
		min, err = strconv.ParseInt(data[0], 10, 0)
		max, err = strconv.ParseInt(data[1], 10, 0)
	}

	errInvalidRange := fmt.Errorf("Incorrect range: %s", s)

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

func (r *IntRange) Set(value string) (err error) {
	r.Min, r.Max, err = ParseIntRange(value)

	if err != nil {
		return err
	}

	if r.Min == 0 {
		return fmt.Errorf("Actual value size cannot be equal to 0")
	}

	return nil
}

func (r IntRange) String() string {
	return fmt.Sprintf("%d-%d", r.Min, r.Max)
}

func (r IntRange) Value() IntRange {
	return r
}

type HwAddr string

func (a *HwAddr) Set(value string) error {
	hwaddr, err := net.ParseMAC(value)

	if err != nil {
		return err
	}

	(*a) = HwAddr(hwaddr.String())

	return nil
}

func (a HwAddr) String() string {
	return string(a)
}

func (a HwAddr) Value() HwAddr {
	return a
}
