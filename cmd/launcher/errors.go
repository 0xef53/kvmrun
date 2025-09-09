package main

import "errors"

var (
	errNotFatal = errors.New("non-fatal error")
)
