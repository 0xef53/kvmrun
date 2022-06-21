package kvmrun

import (
	"fmt"
)

var Version = KvmrunVersion{1, 0, 2}

type KvmrunVersion struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Micro int `json:"micro"`
}

func (v KvmrunVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Micro)
}

func (v KvmrunVersion) ToInt() int {
	return v.Major*10000 + v.Minor*100 + v.Micro
}
