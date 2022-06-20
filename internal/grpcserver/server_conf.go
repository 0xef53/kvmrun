package grpcserver

import (
	"crypto/tls"
	"fmt"
	"net"
)

type ServerConf struct {
	Bindings   []string `gcfg:"listen"  json:"listen"`
	BindSocket string   `gcfg:"-" json:"-"`

	TLSConfig *tls.Config `gcfg:"-" json:"-"`
}

func (c *ServerConf) Listeners() ([]net.Listener, error) {
	var tcpListen func(string) (net.Listener, error)

	if c.TLSConfig == nil {
		tcpListen = func(addr string) (net.Listener, error) { return net.Listen("tcp", addr+":8383") }
	} else {
		tcpListen = func(addr string) (net.Listener, error) { return tls.Listen("tcp", addr+":9393", c.TLSConfig) }
	}

	ipaddrs, err := c.resolve(c.Bindings)
	if err != nil {
		return nil, err
	}

	listeners := make([]net.Listener, 0, len(ipaddrs))

	releaseFn := func() {
		for _, l := range listeners {
			l.Close()
		}
	}

	var success bool

	defer func() {
		if !success {
			releaseFn()
		}
	}()

	for _, ipaddr := range ipaddrs {
		var hostport string

		if ipaddr.To4() != nil {
			hostport = ipaddr.String()
		} else {
			hostport = "[" + ipaddr.String() + "]"
		}

		l, err := tcpListen(hostport)
		if err != nil {
			return nil, err
		}

		listeners = append(listeners, l)
	}

	success = true

	return listeners, nil
}

func (c *ServerConf) BindAddrs() ([]net.IP, error) {
	return c.resolve(c.Bindings)
}

func (c *ServerConf) resolve(bindings []string) ([]net.IP, error) {
	m := make(map[string]net.IP)

	for _, x := range bindings {
		// Try to parse into an IP
		if ip := net.ParseIP(x); ip != nil {
			if _, ok := m[x]; !ok {
				m[x] = ip
			}
			continue
		}

		// Perhaps this is a network interface name
		ifaceIPs, err := getIfaceAddrs(x)
		if err != nil {
			return nil, err
		}
		for _, ip := range ifaceIPs {
			if _, ok := m[ip.String()]; !ok {
				m[ip.String()] = ip
			}
		}
	}

	addrs := make([]net.IP, 0, len(m))

	for _, ip := range m {
		addrs = append(addrs, ip)
	}

	return addrs, nil
}

func getIfaceAddrs(ifname string) ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, netif := range ifaces {
		if netif.Name != ifname {
			continue
		}

		addrs, err := netif.Addrs()
		if err != nil {
			return nil, err
		}

		ips := make([]net.IP, 0, len(addrs))

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.IsLinkLocalUnicast() {
					continue
				}
				ips = append(ips, ipnet.IP)
			}
		}

		return ips, nil
	}

	return nil, fmt.Errorf("no such network interface: %s", ifname)
}
