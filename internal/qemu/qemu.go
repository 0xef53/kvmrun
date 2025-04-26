package qemu

import (
	"errors"
	"fmt"

	"github.com/0xef53/kvmrun/internal/version"
)

var (
	ErrUnsupportedVersion = errors.New("unsupported QEMU version")
)

func VerifyVersion(strver string) error {
	v, err := version.Parse(strver)
	if err != nil {
		return err
	}

	if _, ok := machines[v.Int()]; ok {
		return nil
	}

	return fmt.Errorf("%w: %s", ErrUnsupportedVersion, strver)
}
