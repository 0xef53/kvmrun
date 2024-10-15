package osprober

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type OsReleaseProber struct{}

func (p OsReleaseProber) probe(fname string) (*OSReleaseInfo, error) {
	info := OSReleaseInfo{
		Source: "os-release",
	}

	r := regexp.MustCompile(`^(NAME|ID|VERSION_ID|VERSION_CODENAME|PRETTY_NAME)=(\S+.*)`)

	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)

	for scanner.Scan() {
		fields := r.FindStringSubmatch(scanner.Text())
		if fields == nil {
			continue
		}
		key, value := fields[1], strings.Trim(fields[2], `"`)
		switch key {
		case "NAME":
			info.Name = value
		case "PRETTY_NAME":
			info.PrettyName = value
		case "ID":
			info.Distrib = strings.ToLower(value)
		case "VERSION_ID":
			info.Version = strings.ToLower(value)
		case "VERSION_CODENAME":
			info.CodeName = strings.ToLower(value)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	switch info.Distrib {
	case "opensuse-leap":
		info.Family = "opensuse"
	case "sangoma":
		info.Family = "centos"
	default:
		info.Family = info.Distrib
	}

	if len(info.CodeName) == 0 {
		switch info.Family {
		case "debian":
			if v, ok := DEBIAN_CODES[info.Version]; ok {
				info.CodeName = v
			}
		case "ubuntu":
			parts := strings.Split(info.Version, ".")
			if len(parts) >= 2 {
				if v, ok := UBUNTU_CODES[strings.Join(parts[:2], ".")]; ok {
					info.CodeName = v
				}
			}
		case "opensuse":
			parts := strings.Split(info.Version, ".")
			if len(parts) > 1 {
				info.CodeName = parts[0]
			}
		}
	}

	return &info, nil
}

func (p OsReleaseProber) Probe(rootdir string) (*OSReleaseInfo, error) {
	return p.probe(filepath.Join(rootdir, "etc/os-release"))
}
