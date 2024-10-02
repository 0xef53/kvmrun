package cloudinit

import (
	"net"

	"github.com/0xef53/kvmrun/internal/ipmath"
)

func AutoDefaultRoute(addr *net.IPNet) net.IP {
	if addr == nil {
		return nil
	}

	ones, bits := addr.Mask.Size()

	if addr.IP.To4() != nil {
		// IPv4
		if ones > 30 || (ones == 0 && bits == 0) {
			return net.IPv4(10, 11, 11, 11)
		}

		last, _ := ipmath.GetLastIPv4(addr)

		return last
	}

	// IPv6
	if ones > 64 || (ones == 0 && bits == 0) {
		return nil
	}

	last, _ := ipmath.GetLastIPv6(addr)

	return last
}
