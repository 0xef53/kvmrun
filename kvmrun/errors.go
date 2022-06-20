package kvmrun

import (
	"errors"
	"fmt"
)

var (
	ErrNotImplemented = errors.New("not implemented")
	ErrTimedOut       = errors.New("timeout error")
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

type NotRunningError struct {
	Name string
}

func (e *NotRunningError) Error() string {
	return "not running: " + e.Name
}

func IsNotRunningError(err error) bool {
	if _, ok := err.(*NotRunningError); ok {
		return true
	}

	return false
}

type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string {
	return "not found: " + e.Name
}
