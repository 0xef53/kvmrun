package kvmrun

import (
	"errors"
)

var ErrIncorrectContextID = errors.New("incorrect context ID (allowed range is from 3 to 0xffffffff - 1)")

type VirtioVSock struct {
	Auto      bool   `json:"auto,omitempty"`
	ContextID uint32 `json:"context_id,omitempty"`
	Addr      string `json:"addr,omitempty"`
}
