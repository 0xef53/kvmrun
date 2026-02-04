package backend

import (
	"errors"
	"fmt"
)

var ErrNotImplemented = errors.New("not implemented")

type UnknownBackendError struct {
	Path string
}

func (e *UnknownBackendError) Error() string {
	return fmt.Sprintf("could not determine backend type for device: %s", e.Path)
}

func IsUnknownBackendError(err error) bool {
	if _, ok := err.(*UnknownBackendError); ok {
		return true
	}

	return false
}
