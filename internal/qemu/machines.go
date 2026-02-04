package qemu

import (
	"fmt"

	"github.com/0xef53/kvmrun/internal/version"
)

type machineType struct {
	Name    string `json:"name"`
	Default bool   `json:"default"`
}

var machines = map[int][]machineType{
	30100: {{"pc", true}, {"pc-i440fx-3.1", true}},
	40000: {{"pc", true}, {"pc-i440fx-4.0", true}},
	40001: {{"pc", true}, {"pc-i440fx-4.0.1", true}},
	40100: {{"pc", true}, {"pc-i440fx-4.1", true}},
	40200: {{"pc", true}, {"pc-i440fx-4.2", true}},
	50000: {{"pc", true}, {"pc-i440fx-5.0", true}},
	50100: {{"pc", true}, {"pc-i440fx-5.1", true}},
	50200: {{"pc", true}, {"pc-i440fx-5.2", true}},
	60000: {{"pc", true}, {"pc-i440fx-6.0", true}},
	60100: {{"pc", true}, {"pc-i440fx-6.1", true}},
	60200: {{"pc", true}, {"pc-i440fx-6.2", true}},
	70000: {{"pc", true}, {"pc-i440fx-7.0", true}},
	70100: {{"pc", true}, {"pc-i440fx-7.1", true}},
	70200: {{"pc", true}, {"pc-i440fx-7.2", true}},
	80000: {{"pc", true}, {"pc-i440fx-8.0", true}},
	80100: {{"pc", true}, {"pc-i440fx-8.1", true}},
	80200: {{"pc", true}, {"pc-i440fx-8.2", true}},
	90000: {{"pc", true}, {"pc-i440fx-9.0", true}},
	90100: {{"pc", true}, {"pc-i440fx-9.1", true}},
	90200: {{"pc", true}, {"pc-i440fx-9.2", true}},
}

func GetDefaultMachineType(strver string) (*machineType, error) {
	v, err := version.Parse(strver)
	if err != nil {
		return nil, err
	}

	if mtypes := getSuitableTypes(v); mtypes != nil {
		for _, t := range mtypes {
			if t.Name == "pc" {
				// this is alias to default
				continue
			}
			if t.Default {
				return &t, nil
			}
		}

		if len(mtypes) > 0 {
			return &(mtypes[0]), nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrUnsupportedVersion, strver)
}

func IsDefaultMachineType(strver, mtype string) bool {
	v := version.MustParse(strver)

	if mtypes := getSuitableTypes(v); mtypes != nil {
		for _, t := range mtypes {
			if t.Default && t.Name == mtype {
				return true
			}
		}
	}

	return false
}

func getSuitableTypes(v *version.Version) []machineType {
	if x, ok := machines[v.Int()]; ok {
		return x
	} else {
		if v.Micro != 0 {
			v.Micro = 0

			return getSuitableTypes(v)
		}
	}

	return nil
}
