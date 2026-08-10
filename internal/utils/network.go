package utils

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"
)

func ParseIPNet(s string) (*net.IPNet, error) {
	if !strings.Contains(s, "/") {
		if net.ParseIP(s).To4() != nil {
			s += "/32"
		} else {
			s += "/128"
		}
	}

	return netlink.ParseIPNet(s)
}

func ParseCIDR(s string) (net.IP, *net.IPNet, error) {
	if !strings.Contains(s, "/") {
		if net.ParseIP(s).To4() != nil {
			s += "/32"
		} else {
			s += "/128"
		}
	}

	return net.ParseCIDR(s)
}

func GetRouteTableIndex(table string) (int, error) {
	var errTableNotFound = errors.New("table not found")

	findIn := func(fname string) (int, error) {
		fd, err := os.Open(fname)
		if err != nil {
			return -1, err
		}
		defer fd.Close()

		scanner := bufio.NewScanner(fd)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "#") {
				continue
			}

			ff := strings.Fields(line)

			if len(ff) == 2 && strings.ToLower(ff[1]) == table {
				if v, err := strconv.Atoi(ff[0]); err == nil {
					return v, nil
				} else {
					return -1, err
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return -1, err
		}

		return -1, errTableNotFound
	}

	possiblePlaces := []string{
		"/etc/iproute2/rt_tables",
		"/usr/share/iproute2/rt_tables",
	}

	// Also look in /etc/iproute2/rt_tables.d/*
	if ff, err := os.ReadDir("/etc/iproute2/rt_tables.d"); err == nil {
		for _, f := range ff {
			if f.Type().IsRegular() {
				possiblePlaces = append(possiblePlaces, filepath.Join("/etc/iproute2/rt_tables.d", f.Name()))
			}
		}
	} else {
		if !os.IsNotExist(err) {
			return -1, err
		}
	}

	for _, p := range possiblePlaces {
		idx, err := findIn(p)
		if err != nil {
			if os.IsNotExist(err) || err == errTableNotFound {
				continue
			}
			return -1, err
		}

		return idx, nil
	}

	return -1, fmt.Errorf("%w: %s", errTableNotFound, table)
}

func IntToIPv4(n uint32) net.IP {
	ip := make(net.IP, net.IPv4len)

	binary.BigEndian.PutUint32(ip, n)

	return ip
}

func IPv4ToInt(ip net.IP) (uint32, error) {
	if ip.To4() == nil {
		return 0, fmt.Errorf("not an IPv4 address: %s", ip.String())
	}

	return binary.BigEndian.Uint32(ip.To4()), nil
}
