package network

import (
	"fmt"
	"net"
)

func GetBindAddrs(x string) ([]net.IP, error) {
	// Try to parse into an IP
	if ip := net.ParseIP(x); ip != nil {
		return []net.IP{ip}, nil
	}

	// Perhaps this is a network interface name
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, netif := range ifaces {
		if netif.Name == x {
			addrs, err := netif.Addrs()
			if err != nil {
				return nil, err
			}

			ips := make([]net.IP, 0, len(addrs))

			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok {
					if ipnet.IP.To4() != nil && !ipnet.IP.IsLinkLocalUnicast() {
						ips = append(ips, ipnet.IP)
					}
				}
			}

			return ips, nil
		}
	}

	return nil, fmt.Errorf("no such network interface: %s", x)
}
