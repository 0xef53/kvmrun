package cloudinit

import (
	"net"

	"github.com/0xef53/kvmrun/internal/ipmath"
)

// AutoDefaultRoute returns an IP of the default gateway,
// calculated according to the following rules:
//   - IPv4, netlen < 32:  the last addr from the network
//   - IPv4, netlen = 32:  onlink 10.11.11.11 (must be configured
//     on dummy-interface on the host)
//   - IPv6:               the last addr from the network
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
