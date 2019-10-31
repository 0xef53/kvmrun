//+build linux

// Package go-cgroup provides primitives to work with Linux Control Groups
// via pseudo file system /sys/fs/cgroup.
//
// https://www.kernel.org/doc/Documentation/cgroup-v1/cgroups.txt

package cgroup

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cgroup is an uniform interface for the cgroups.
type Cgroup interface {
	// Sets the cgroup parameters represented by *Config variable
	Set(*Config) error

	// Gets the actual cgroup parameters and stores them to the pointer of *Config
	Get(*Config) error

	// Gets the cgroup stat information and store its to the pointer of *Stats
	GetStats(*Stats) error

	// Returns full path of the cgroup relative to the filesystem root
	GetPath() string
}

// LookupCgroupByPid tries to find the group containing the specified pid.
//
// If the specified subsystem is not supported by this package, an error of type
// *UnsupportedSubsystemError will be returned.
func LookupCgroupByPid(pid int, subsystem string) (Cgroup, error) {
	subsystemPath, err := GetSubsystemMountpoint(subsystem)
	if err != nil {
		return nil, err
	}

	subpath, err := GetCgroupPathByPid(pid, subsystem)
	if err != nil {
		return nil, err
	}

	path := filepath.Join(subsystemPath, subpath)

	var g Cgroup

	switch subsystem {
	case "cpu":
		g = Cgroup(&CpuGroup{path})
	default:
		return nil, NewUnsupportedError(subsystem)
	}

	return g, nil
}

// DestroyCgroup destroys the cgroup located on the given path.
func DestroyCgroup(path string) error {
	os.RemoveAll(path)

	// RemoveAll always returns error, even on already removed path.
	// This occurs when we trying to remove files from the group directory.
	// That's why there is next strange test.
	if _, err := os.Stat(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// GetEnabledSubsystems returns a map with all the subsystems supported by the kernel.
func GetEnabledSubsystems() (map[string]int, error) {
	cgroupsFile, err := os.Open("/proc/cgroups")
	if err != nil {
		return nil, err
	}
	defer cgroupsFile.Close()

	scanner := bufio.NewScanner(cgroupsFile)

	// Skip the first line. It's a comment
	scanner.Scan()

	cgroups := make(map[string]int)
	for scanner.Scan() {
		var subsystem string
		var hierarchy int
		var num int
		var enabled int
		fmt.Sscanf(scanner.Text(), "%s %d %d %d", &subsystem, &hierarchy, &num, &enabled)

		if enabled == 1 {
			cgroups[subsystem] = hierarchy
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Cannot parsing /proc/cgroups: %s", err)
	}

	return cgroups, nil
}

// GetSubsystemMountpoint returns a path where a given subsystem is mounted.
func GetSubsystemMountpoint(subsystem string) (string, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		txt := scanner.Text()
		fields := strings.Split(txt, " ")
		for _, opt := range strings.Split(fields[len(fields)-1], ",") {
			if opt == subsystem {
				return fields[4], nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", NewMountpointError(subsystem)
}

// GetCgroupPathByPid returns a path to the cgroup containing the specified pid.
func GetCgroupPathByPid(pid int, subsystem string) (string, error) {
	cgroups, err := GetProcessCgroups(pid)
	if err != nil {
		return "", err
	}

	for s, p := range cgroups {
		if s == subsystem {
			return p, nil
		}
	}

	return "", NewNotInSubsystemError(pid, subsystem)
}

// GetProcessCgroups returns a map with the all cgroup subsystems
// and their relative paths to which the specified pid belongs.
func GetProcessCgroups(pid int) (map[string]string, error) {
	fname := fmt.Sprintf("/proc/%d/cgroup", pid)

	cgroups := make(map[string]string)

	f, err := os.Open(fname)
	if err != nil {
		return nil, fmt.Errorf("Cannot open %s: %s", fname, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("Cannot parsing %s: unknown format", fname)
		}
		subsystemsParts := strings.Split(parts[1], ",")
		for _, s := range subsystemsParts {
			cgroups[s] = parts[2]
		}
	}

	if len(cgroups) == 0 {
		return nil, NewCgroupsNotFoundError(pid)
	}

	return cgroups, nil
}

type UnsupportedError struct {
	Subsystem string
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("Unsupported subsystem: %s", e.Subsystem)
}

func NewUnsupportedError(subsystem string) error {
	return &UnsupportedError{subsystem}
}

func IsUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*UnsupportedError)
	return ok
}

type MountpointError struct {
	subsystem string
}

func (e *MountpointError) Error() string {
	return fmt.Sprintf("Mountpoint not found: subsystem = %s", e.subsystem)
}

func NewMountpointError(subsystem string) error {
	return &MountpointError{subsystem}
}

func IsMountpointError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*MountpointError)
	return ok
}

type CgroupsNotFoundError struct {
	pid int
}

func (e *CgroupsNotFoundError) Error() string {
	return fmt.Sprintf("No one control group found: PID = %d", e.pid)
}

func NewCgroupsNotFoundError(pid int) error {
	return &CgroupsNotFoundError{pid}
}

func IsCgroupsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*CgroupsNotFoundError)
	return ok
}

type NotInSubsystemError struct {
	pid       int
	subsystem string
}

func (e *NotInSubsystemError) Error() string {
	return fmt.Sprintf("Not in subsystem: PID = %d, subsystem = %s", e.pid, e.subsystem)
}

func NewNotInSubsystemError(pid int, subsystem string) error {
	return &NotInSubsystemError{pid, subsystem}
}

func IsNotInSubsystemError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*NotInSubsystemError)
	return ok
}
