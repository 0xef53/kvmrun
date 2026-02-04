package flag_types

import (
	"fmt"
	"strings"
)

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

func (t StringMap) Get() interface{} {
	return t.m
}
