package kvmrun

import (
	"errors"
	"fmt"
)

const (
	INCOMINGPORT = 30000
	WEBSOCKSPORT = 10700
	NBDPORT      = 60000

	VMCONFDIR = "/etc/kvmrun"
	SVDATADIR = "/var/lib/supervise"

	VMNETINIT = "/usr/lib/kvmrun/netinit"

	QMPMONDIR  = "/var/run/kvm-monitor"
	CHROOTDIR  = "/var/lib/kvmrun/chroot"
	KERNELSDIR = "/var/lib/kvmrun/kernels"
	MODULESDIR = "/var/lib/kvmrun/modules"
	LOGSDIR    = "/var/log/kvmrun"
)

var (
	DEBUG bool

	ErrNotImplemented = errors.New("Not implemented")
	ErrTimedOut       = errors.New("Timeout error")
)

type DevDriver struct {
	Name         string `json:"name"`
	HotPluggable bool   `json:"hotplugguble"`
}

type DevDrivers []DevDriver

func (d DevDrivers) Exists(name string) bool {
	for _, v := range d {
		if v.Name == name {
			return true
		}
	}
	return false
}

func (d DevDrivers) HotPluggable(name string) bool {
	for _, v := range d {
		if v.Name == name && v.HotPluggable {
			return true
		}
	}
	return false
}

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
	VMName string
}

func (e *NotRunningError) Error() string {
	return "Not running: " + e.VMName
}

type NotFoundError struct {
	VMName string
}

func (e *NotFoundError) Error() string {
	return "Not found: " + e.VMName
}
