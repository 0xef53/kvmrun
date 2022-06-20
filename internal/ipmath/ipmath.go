package ipmath

import (
	"encoding/binary"
	"fmt"
	"net"
)

func mask32(nbits int) uint32 {
	return -uint32(1 << uint(32-nbits))
}

func GetLastIPv4(ipnet *net.IPNet) (net.IP, error) {
	if ipnet.IP.To4() == nil {
		return nil, fmt.Errorf("not an IPv4 address")
	}

	nbits, _ := ipnet.Mask.Size()

	ipInt := binary.BigEndian.Uint32(ipnet.IP.To4())

	ip := make(net.IP, net.IPv4len)

	binary.BigEndian.PutUint32(ip, ipInt|^mask32(nbits))

	return ip, nil
}

func mask64(nbits int) uint64 {
	return -uint64(1 << uint(64-nbits))
}

func invmask(nbits int) [2]uint64 {
	var m [2]uint64

	if nbits > 64 {
		m[0], m[1] = ^mask64(64), ^mask64(nbits-64)
	} else {
		m[0], m[1] = ^mask64(nbits), mask64(64)
	}

	return m
}
func GetLastIPv6(ipnet *net.IPNet) (net.IP, error) {
	if !(ipnet.IP.To16() != nil && ipnet.IP.To4() == nil) {
		return nil, fmt.Errorf("not an IPv6 address")
	}

	nbits, _ := ipnet.Mask.Size()

	ipInt := [2]uint64{
		binary.BigEndian.Uint64(ipnet.IP[:8]),
		binary.BigEndian.Uint64(ipnet.IP[8:16]),
	}

	mInt := invmask(nbits)

	ip := make(net.IP, net.IPv6len)

	binary.BigEndian.PutUint64(ip[:8], ipInt[0]|mInt[0])
	binary.BigEndian.PutUint64(ip[8:16], ipInt[1]|mInt[1])

	return ip, nil
}
