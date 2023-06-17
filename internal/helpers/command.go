package helpers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func CommandExitCode(err error) (int, bool) {
	if err == nil {
		return 0, true
	}

	var exitCode int

	if exiterr, ok := err.(*exec.ExitError); ok {
		status := exiterr.Sys().(syscall.WaitStatus)

		switch {
		case status.Exited():
			exitCode = status.ExitStatus()
		case status.Signaled():
			exitCode = 128 + int(status.Signal())
		}
	} else {
		return 1, false
	}

	return exitCode, true
}

func ResolveExecutable(fname string) (string, error) {
	st, err := os.Stat(fname)
	if err != nil {
		return "", err
	}

	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("not a file: %s", fname)
	}

	if st.Mode()&0100 == 0 {
		return "", fmt.Errorf("not executable by root: %s", fname)
	}

	return filepath.Abs(fname)
}
