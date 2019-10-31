package kcfg

import (
	"fmt"
	"gopkg.in/gcfg.v1"
	"net"
	"path/filepath"
)

type CommonParams struct {
	CertDir   string `gcfg:"cert-dir"`
	CACrt     string `gcfg:"-"`
	CAKey     string `gcfg:"-"`
	ServerCrt string `gcfg:"-"`
	ServerKey string `gcfg:"-"`
	ClientCrt string `gcfg:"-"`
	ClientKey string `gcfg:"-"`
}

type ServerParams struct {
	Bindings  []string `gcfg:"listen"`
	BindAddrs []net.IP `gcfg:"-"`
}

// KvmrunConfig represents the Kvmrun configuration.
type KvmrunConfig struct {
	Common CommonParams
	Server ServerParams
}

// NewConfig reads and parses the configuration file and returns
// a new instance of KvmrunConfig on success.
func NewConfig(p string) (*KvmrunConfig, error) {
	cfg := KvmrunConfig{
		Common: CommonParams{
			CertDir: "/usr/share/kvmrun/tls",
		},
	}

	err := gcfg.ReadFileInto(&cfg, p)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse config file: %s", err)
	}

	cfg.Common.CACrt = filepath.Join(cfg.Common.CertDir, "CA.crt")
	cfg.Common.CAKey = filepath.Join(cfg.Common.CertDir, "CA.key")
	cfg.Common.ServerCrt = filepath.Join(cfg.Common.CertDir, "server.crt")
	cfg.Common.ServerKey = filepath.Join(cfg.Common.CertDir, "server.key")
	cfg.Common.ClientCrt = filepath.Join(cfg.Common.CertDir, "client.crt")
	cfg.Common.ClientKey = filepath.Join(cfg.Common.CertDir, "client.key")

	ips := make(map[string]struct{})

	for _, s := range cfg.Server.Bindings {
		// Try to parse into an IP
		if ip := net.ParseIP(s); ip != nil {
			if _, ok := ips[s]; !ok {
				ips[s] = struct{}{}
			}
			continue
		}
		// Perhaps this is a network interface name
		ifaceIPs, err := getIfaceAddrs(s)
		if err != nil {
			return nil, err
		}
		for _, ip := range ifaceIPs {
			if _, ok := ips[ip]; !ok {
				ips[ip] = struct{}{}
			}
		}
	}

	for ip := range ips {
		cfg.Server.BindAddrs = append(cfg.Server.BindAddrs, net.ParseIP(ip))
	}

	return &cfg, nil
}

func getIfaceAddrs(ifname string) ([]string, error) {
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

		ips := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.IsLinkLocalUnicast() {
					continue
				}
				ips = append(ips, ipnet.IP.String())
			}
		}

		return ips, nil
	}

	return nil, fmt.Errorf("No such network interface: %s", ifname)
}
