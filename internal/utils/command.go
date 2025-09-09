package utils

import (
	"os/exec"
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
