package kvmrun

import (
	"fmt"
)

var VERSION = VersionInfo{0, 3, 2}

type VersionInfo struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Micro int `json:"micro"`
}

func (v VersionInfo) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Micro)
}

func (v VersionInfo) ToInt() int {
	return v.Major*10000 + v.Minor*100 + v.Micro
}
