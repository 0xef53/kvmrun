package qemu

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/0xef53/kvmrun/internal/version"
)

var (
	ErrUnsupportedVersion = errors.New("unsupported QEMU version")
)

func VerifyVersion(strver string) error {
	v, err := version.Parse(strver)
	if err != nil {
		return err
	}

	if _, ok := machines[v.Int()]; ok {
		return nil
	}

	return fmt.Errorf("%w: %s", ErrUnsupportedVersion, strver)
}

func GetVersion(rootdir, binary string) (*version.Version, error) {
	qemuCommand := exec.Command(binary, "-version")

	qemuCommand.Env = append(qemuCommand.Environ(), fmt.Sprintf("QEMU_ROOTDIR=%s", rootdir))

	out, err := qemuCommand.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("QEMU binary failed (%s): %s", err, strings.TrimSpace(string(out)))
	}

	r := regexp.MustCompile(`^qemu\semulator\sversion\s([0-9\.]{3,})`)

	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		line := strings.ToLower(strings.TrimSpace(scanner.Text()))

		fields := r.FindStringSubmatch(line)

		if len(fields) == 2 {
			return version.Parse(fields[1])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("could not determine QEMU version: bad output")
}
