package kvmrun

import (
	"errors"
	"fmt"
)

var (
	ErrAlreadyExists  = errors.New("already exists")
	ErrNotFound       = errors.New("not found")
	ErrNotRunning     = errors.New("machine not running")
	ErrNotImplemented = errors.New("not implemented")
)

type AlreadyConnectedError struct {
	Source string
	Object string
}

func (e *AlreadyConnectedError) Error() string {
	return fmt.Sprintf("%s: object already connected: %s", e.Source, e.Object)
}

func IsAlreadyConnectedError(err error) bool {
	if _, ok := err.(*AlreadyConnectedError); ok {
		return true
	}

	return false
}

type NotConnectedError struct {
	Source string
	Object string
}

func (e *NotConnectedError) Error() string {
	return fmt.Sprintf("%s: object not found: %s", e.Source, e.Object)
}

func IsNotConnectedError(err error) bool {
	if _, ok := err.(*NotConnectedError); ok {
		return true
	}

	return false
}
