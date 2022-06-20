package netstat

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Linux Socket states
type SockState uint8

const (
	Established SockState = iota + 1
	SynSent
	SynRecv
	FinWait1
	FinWait2
	TimeWait
	Close
	CloseWait
	LastAck
	Listen
	Closing
)

func tcpSocks(fn FilterFn) ([]SockTableEntry, error) {
	var entries []SockTableEntry

	if ee, err := tcpSocks4(fn); err == nil {
		entries = append(entries, ee...)
	} else {
		return nil, err
	}

	if ee, err := tcpSocks6(fn); err == nil {
		entries = append(entries, ee...)
	} else {
		return nil, err
	}

	return entries, nil
}

func tcpSocks4(fn FilterFn) ([]SockTableEntry, error) {
	return getStat("/proc/net/tcp", fn)
}

func tcpSocks6(fn FilterFn) ([]SockTableEntry, error) {
	return getStat("/proc/net/tcp6", fn)
}

func getStat(statfile string, fn FilterFn) ([]SockTableEntry, error) {
	entries, err := parseSockTable(statfile, fn)
	if err != nil {
		return nil, err
	}

	if err := setProcessInfo(entries); err != nil {
		return nil, err
	}

	return entries, nil
}

func parseSockTable(statfile string, ok FilterFn) ([]SockTableEntry, error) {
	entries := make([]SockTableEntry, 0, 5)

	fd, err := os.Open(statfile)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)

	// Skip the first line -- title
	scanner.Scan()

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "#") {
			continue
		}

		fields := strings.Fields(scanner.Text())

		if len(fields) < 12 {
			return nil, fmt.Errorf("not enough fields: %v, %v", len(fields), fields)
		}

		laddr, err := parseAddr(fields[1])
		if err != nil {
			return nil, err
		}

		raddr, err := parseAddr(fields[2])
		if err != nil {
			return nil, err
		}

		entry := SockTableEntry{
			Inode:      fields[9],
			LocalAddr:  laddr,
			RemoteAddr: raddr,
		}

		// State
		if v, err := strconv.ParseUint(fields[3], 16, 8); err == nil {
			entry.State = SockState(v)
		} else {
			return nil, err
		}

		// UID
		if v, err := strconv.ParseUint(fields[7], 10, 32); err == nil {
			entry.UID = uint32(v)
		} else {
			return nil, err
		}

		if ok(&entry) {
			entries = append(entries, entry)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func parseAddr(s string) (*SockAddr, error) {
	fields := strings.Split(s, ":")
	if len(fields) < 2 {
		return nil, fmt.Errorf("netstat: not enough fields: %v", s)
	}

	var parse func(string) (net.IP, error)

	switch len(fields[0]) {
	case 8:
		parse = parseIPv4
	case 32:
		parse = parseIPv6
	default:
		return nil, fmt.Errorf("invalid ip:port string: %v", fields[0])
	}

	ip, err := parse(fields[0])
	if err != nil {
		return nil, err
	}

	port, err := strconv.ParseUint(fields[1], 16, 16)
	if err != nil {
		return nil, err
	}

	return &SockAddr{IP: ip, Port: uint16(port)}, nil
}

func parseIPv4(s string) (net.IP, error) {
	ip := make(net.IP, net.IPv4len)

	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return nil, err
	}

	binary.LittleEndian.PutUint32(ip, uint32(v))

	return ip, nil
}

func parseIPv6(s string) (net.IP, error) {
	const groups = 4

	ip := make(net.IP, net.IPv6len)

	i, j := 0, 4

	for len(s) != 0 {
		v, err := strconv.ParseUint(s[0:8], 16, 32)
		if err != nil {
			return nil, err
		}

		binary.LittleEndian.PutUint32(ip[i:j], uint32(v))

		i, j = i+groups, j+groups

		s = s[8:]
	}

	return ip, nil
}

func setProcessInfo(entries []SockTableEntry) error {
	handle := func(pid int) error {
		basedir := filepath.Join("/proc", strconv.Itoa(pid))

		// format of link name: socket:[5860846]
		fds, err := ioutil.ReadDir(filepath.Join(basedir, "fd"))
		if err != nil {
			return err
		}

		for _, fdfile := range fds {
			lname, err := os.Readlink(filepath.Join(basedir, "fd", fdfile.Name()))
			if err != nil || !strings.HasPrefix(lname, "socket:[") {
				continue
			}

			for idx := range entries {
				e := &entries[idx]

				if lname == "socket:["+e.Inode+"]" && e.Process == nil {
					b, err := ioutil.ReadFile(filepath.Join(basedir, "comm"))
					if err != nil {
						return err
					}

					e.Process = &Process{pid, strings.TrimSpace(string(b))}
				}
			}
		}

		return nil
	}

	files, err := ioutil.ReadDir("/proc")
	if err != nil {
		return err
	}

	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(f.Name())
		if err != nil {
			continue
		}

		if err := handle(pid); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}
