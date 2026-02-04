package lvm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func IsLogicalVolume(p string) (bool, error) {
	dmName, err := filepath.EvalSymlinks(p)
	if err != nil {
		return false, err
	}
	dmDir := filepath.Join("/sys/block", filepath.Base(dmName), "dm")

	switch _, err := os.Stat(dmDir); {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	}
	return false, err
}

func CreateVolume(vgname, lvname string, size uint64) error {
	devpath := fmt.Sprintf("/dev/%s/%s", vgname, lvname)

	var st unix.Stat_t

	switch err := unix.Stat(devpath, &st); {
	case err == nil:
		if (st.Mode & unix.S_IFMT) != unix.S_IFBLK { // S_IFMT -- type of file
			return fmt.Errorf("path exists but is not a block device: %s", devpath)
		}
		return &os.PathError{Op: "lvcreate", Path: devpath, Err: os.ErrExist}
	case os.IsNotExist(err):
	default:
		return err
	}

	out, err := exec.Command(
		"/sbin/lvm",
		"lvcreate",
		"--name", lvname,
		"--size", fmt.Sprintf("%dB", size),
		vgname,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("lvcreate failed (%s): %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

func RemoveVolume(devpath string) error {
	if !strings.HasPrefix(devpath, "/dev/") {
		devpath = filepath.Join("/dev/", devpath)
	}

	var st unix.Stat_t

	switch err := unix.Stat(devpath, &st); {
	case err == nil:
		if (st.Mode & unix.S_IFMT) != unix.S_IFBLK { // S_IFMT -- type of file
			return fmt.Errorf("path exists but is not a block device: %s", devpath)
		}
	case os.IsNotExist(err):
		return &os.PathError{Op: "lvremove", Path: devpath, Err: os.ErrNotExist}
	default:
		return err
	}

	out, err := exec.Command("/sbin/lvm", "lvremove", "-f", devpath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("lvremove failed (%s): %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}
