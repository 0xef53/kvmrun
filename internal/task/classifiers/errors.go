package classifiers

import "errors"

var (
	ErrValidationFailed  = errors.New("validation error")
	ErrAssignmentFailed  = errors.New("cannot assign")
	ErrAssignmentTimeout = errors.New("assignment timeout")
)
