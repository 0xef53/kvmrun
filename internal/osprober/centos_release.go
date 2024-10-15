package osprober

import (
	"os"
	"path/filepath"
	"strings"
)

type CentosReleaseProber struct{}

func (p CentosReleaseProber) probe(fname string) (*OSReleaseInfo, error) {
	info := OSReleaseInfo{
		Source:  "centos-release",
		Family:  "centos",
		Distrib: "centos",
		Name:    "CentOS Linux",
	}

	b, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	// LSB format: "distro release x.x (codename)"
	// or
	// Pre-LSB format: "distro x.x (codename)"

	verstr := strings.ToLower(strings.TrimSpace(string(b)))
	parts := strings.Fields(verstr)
	if len(parts) >= 3 {
		info.Version = strings.Split(parts[len(parts)-2], ".")[0]
	}

	info.PrettyName = verstr

	return &info, nil
}

func (p CentosReleaseProber) Probe(rootdir string) (*OSReleaseInfo, error) {
	return p.probe(filepath.Join(rootdir, "etc/centos-release"))
}
