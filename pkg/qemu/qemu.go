package qemu

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	BINARY = "/usr/bin/qemu-system-x86_64"
)

func DefaultMachineType() (string, error) {
	if _, err := os.Stat(BINARY); os.IsNotExist(err) {
		return "", fmt.Errorf("Qemu binary not found: %s", BINARY)
	}
	outBytes, err := exec.Command(BINARY, "-M", "help").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(string(outBytes))
	}
	lines := strings.Split(string(outBytes), "\n")
	for _, line := range lines {
		if strings.HasSuffix(line, "(default)") {
			return strings.Fields(line)[0], nil
		}
	}
	return "", fmt.Errorf("Cannot determine default qemu machine type")
}
