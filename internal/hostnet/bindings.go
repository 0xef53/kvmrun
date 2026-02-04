package hostnet

import (
	"net"
	"slices"
)

// ParseBindings converts a string slice of IP addresses and interface names
// into a slice of [net.IP].
// Interface names are expanded into the set of IP addresses configured on the interface
// at the time the code is executed.
func ParseBindings(bindings ...string) ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	m := make(map[string]net.IP)

	for _, v := range bindings {
		idx := slices.IndexFunc(ifaces, func(iface net.Interface) bool { return iface.Name == v })

		if idx < 0 {
			// Try to parse as IP address
			if ip := net.ParseIP(v); ip != nil {
				if _, ok := m[ip.String()]; !ok {
					m[ip.String()] = ip
				}

				continue
			}
		} else {
			// Perhaps this is a network interface name
			addrs, err := ifaces[idx].Addrs()
			if err != nil {
				return nil, err
			}

			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok {
					if ipnet.IP.IsLinkLocalUnicast() {
						continue
					}

					m[ipnet.IP.String()] = ipnet.IP
				}
			}
		}
	}

	addrs := make([]net.IP, 0, len(m))

	for _, ip := range m {
		addrs = append(addrs, ip)
	}

	return addrs, nil
}
