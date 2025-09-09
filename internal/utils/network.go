package utils

import (
	"bufio"
	"fmt"
	"net"
	"os"
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

func GetRouteTableIndex(table string) (int, error) {
	fd, err := os.Open("/etc/iproute2/rt_tables")
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

	return -1, fmt.Errorf("table not found: %s", table)
}
