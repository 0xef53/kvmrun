package osprober

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
)

type DebianVersionProber struct{}

func (p DebianVersionProber) probe(fname string) (*OSReleaseInfo, error) {
	info := OSReleaseInfo{
		Source:  "debian_version",
		Family:  "debian",
		Distrib: "debian",
		Name:    "Debian GNU/Linux",
	}

	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	verstr := strings.ToLower(strings.TrimSpace(string(b)))

	if strings.Contains(verstr, "/") {
		info.Version = strings.Split(verstr, "/")[0]
	} else {
		parts := strings.Split(verstr, ".")
		if len(parts) > 0 {
			info.Version = parts[0]
		}
		if _, err := strconv.Atoi(info.Version); err != nil {
			info.Version = VerByCode(DEBIAN_CODES, info.Version)
		}
	}

	if v, ok := DEBIAN_CODES[info.Version]; ok {
		info.CodeName = v
	}

	info.PrettyName = fmt.Sprintf("Debian GNU/Linux %s (%s)", info.Version, info.CodeName)

	return &info, nil
}

func (p DebianVersionProber) Probe(rootdir string) (*OSReleaseInfo, error) {
	return p.probe(filepath.Join(rootdir, "etc/debian_version"))
}
