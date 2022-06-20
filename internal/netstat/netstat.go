package netstat

import (
	"fmt"
	"net"
)

type SockTableEntry struct {
	Inode      string
	LocalAddr  *SockAddr
	RemoteAddr *SockAddr
	State      SockState
	UID        uint32
	Process    *Process
}

type SockAddr struct {
	IP   net.IP
	Port uint16
}

func (s *SockAddr) String() string {
	return fmt.Sprintf("%v:%d", s.IP, s.Port)
}

type Process struct {
	Pid  int
	Name string
}

func (p *Process) String() string {
	return fmt.Sprintf("%d/%s", p.Pid, p.Name)
}

type FilterFn func(*SockTableEntry) bool

func NoopFilter(*SockTableEntry) bool { return true }

func TCPSocks(fn FilterFn) ([]SockTableEntry, error) {
	return tcpSocks(fn)
}

func TCPSocks4(fn FilterFn) ([]SockTableEntry, error) {
	return tcpSocks4(fn)
}

func TCPSocks6(fn FilterFn) ([]SockTableEntry, error) {
	return tcpSocks6(fn)
}
