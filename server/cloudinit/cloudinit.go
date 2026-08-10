package cloudinit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xef53/kvmrun/internal/cloudinit"
	"github.com/0xef53/kvmrun/internal/ipmath"
	"github.com/0xef53/kvmrun/internal/utils"
	"github.com/0xef53/kvmrun/kvmrun"

	"gopkg.in/yaml.v3"
)

type CloudInitOptions struct {
	// The instance-data keys
	Platform         string `json:"platform"`
	Subplatform      string `json:"subplatform"`
	Cloudname        string `json:"cloudname"`
	Region           string `json:"region"`
	AvailabilityZone string `json:"availability_zone"`

	// These fields override the same name keys
	// from user_config structure
	Hostname string `json:"hostname"`
	Domain   string `json:"domain"`
	Timezone string `json:"timezone"`

	VendorConfig []byte `json:"vendor_config"`
	UserConfig   []byte `json:"user_config"`
}

func (o *CloudInitOptions) Validate(strict bool) error {
	o.Hostname = strings.TrimSpace(o.Hostname)
	o.Domain = strings.TrimSpace(o.Domain)
	o.Timezone = strings.TrimSpace(o.Timezone)

	if strict {
		var v interface{}

		if err := yaml.Unmarshal(o.VendorConfig, &v); err != nil {
			return fmt.Errorf("invalid vendor-data struct: %w", err)
		}

		if err := yaml.Unmarshal(o.UserConfig, &v); err != nil {
			return fmt.Errorf("invalid user-data struct: %w", err)
		}
	}

	return nil
}

func (s *Server) BuildImage(ctx context.Context, vmname string, opts *CloudInitOptions, outfile string) (string, error) {
	err := func() error {
		// Check if machine exists
		if _, err := kvmrun.GetInstanceConf(vmname); err != nil {
			return err
		}

		vmdir := filepath.Join(kvmrun.CONFDIR, vmname)

		if opts == nil {
			return fmt.Errorf("empty cloudinit opts")
		} else {
			if err := opts.Validate(true); err != nil {
				return err
			}
		}

		outfile = strings.TrimSpace(outfile)

		if len(outfile) == 0 {
			outfile = filepath.Join(kvmrun.CONFDIR, vmname, "config_cidata")
		}

		data := cloudinit.Data{
			Meta: cloudinit.MetadataConfig{
				DSMode:           "local",
				InstanceID:       fmt.Sprintf("i-%s", vmname),
				LocalHostname:    strings.ReplaceAll(vmname, "_", "-"),
				Platform:         opts.Platform,
				Subplatform:      opts.Subplatform,
				Cloudname:        opts.Cloudname,
				Region:           opts.Region,
				AvailabilityZone: opts.AvailabilityZone,
			},
			Network: cloudinit.NetworkConfig{
				Version: 2,
			},
			Hostname: opts.Hostname,
			Domain:   opts.Domain,
			Timezone: opts.Timezone,
		}

		if len(opts.VendorConfig) > 0 {
			if err := yaml.Unmarshal(opts.VendorConfig, &data.Vendor); err != nil {
				return err
			}
		}

		if len(opts.UserConfig) > 0 {
			if err := yaml.Unmarshal(opts.UserConfig, &data.User); err != nil {
				return err
			}
		}

		if v, err := buildEthernetsConfig(vmdir); err == nil {
			data.Network.Ethernets = v
		} else {
			return err
		}

		return cloudinit.GenImage(&data, outfile)
	}()

	if err != nil {
		return "", fmt.Errorf("cannot build cloudinit image: %w", err)
	}

	return outfile, nil
}

func buildEthernetsConfig(vmdir string) (map[string]cloudinit.EthernetConfig, error) {
	macaddrs, err := func() (map[string]string, error) {
		tmp := struct {
			Network []struct {
				Name   string `json:"ifname"`
				Hwaddr string `json:"hwaddr"`
			} `json:"network"`
		}{}

		b, err := os.ReadFile(filepath.Join(vmdir, "config"))
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(b, &tmp); err != nil {
			return nil, err
		}

		m := make(map[string]string)

		for _, netif := range tmp.Network {
			m[netif.Name] = netif.Hwaddr
		}

		return m, nil
	}()
	if err != nil {
		return nil, err
	}

	ethernets, err := func() (map[string]cloudinit.EthernetConfig, error) {
		tmp := []struct {
			ID       string   `json:"id"`
			Name     string   `json:"ifname"`
			Scheme   string   `json:"scheme"`
			Addrs    []string `json:"addrs"`
			Gateway4 string   `json:"gateway4"`
			Gateway6 string   `json:"gateway6"`

			// Deprecated
			DEPRECATED_DefaultGateway string `json:"default_gateway"`
		}{}

		b, err := os.ReadFile(filepath.Join(vmdir, "config_network"))
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &tmp); err != nil {
			return nil, err
		}

		m := make(map[string]cloudinit.EthernetConfig)

		for _, netif := range tmp {
			if _, ok := macaddrs[netif.Name]; !ok || len(netif.Addrs) == 0 {
				// skip unknown devices and without IPs
				continue
			}

			netif.ID = strings.TrimSpace(netif.ID)

			if len(netif.Gateway4) == 0 && len(netif.DEPRECATED_DefaultGateway) > 0 {
				netif.Gateway4 = netif.DEPRECATED_DefaultGateway
			}

			netif.Gateway4 = strings.TrimSpace(netif.Gateway4)
			netif.Gateway6 = strings.TrimSpace(netif.Gateway6)

			var ip4addrs, ip6addrs []*net.IPNet
			var gateway4, gateway6 net.IP

			for _, ipstr := range netif.Addrs {
				ipnet, err := utils.ParseIPNet(ipstr)
				if err != nil {
					return nil, err
				}

				if ipnet.IP.To4() != nil {
					ip4addrs = append(ip4addrs, ipnet)
				} else {
					ip6addrs = append(ip6addrs, ipnet)
				}
			}

			if len(ip4addrs) > 0 && len(netif.Gateway4) > 0 {
				if netif.Gateway4 == "auto" {
					gateway4 = AutoDefaultRoute(ip4addrs[0])
				} else {
					gateway4 = net.ParseIP(netif.Gateway4)
				}
			}

			if len(ip6addrs) > 0 && len(netif.Gateway6) > 0 {
				if netif.Gateway6 == "auto" {
					gateway6 = AutoDefaultRoute(ip6addrs[0])
				} else {
					gateway6 = net.ParseIP(netif.Gateway6)
				}
			}

			cfg := cloudinit.EthernetConfig{
				Addresses: netif.Addrs,
			}

			cfg.Match.MacAddress = macaddrs[netif.Name]

			if gateway4 != nil {
				cfg.Gateway4 = gateway4.String()
			}

			if gateway6 != nil {
				cfg.Gateway6 = gateway6.String()
			}

			if len(netif.ID) > 0 {
				m[netif.ID] = cfg
			} else {
				m[fmt.Sprintf("net-%s", netif.Name)] = cfg
			}
		}

		return m, nil
	}()
	if err != nil {
		return nil, err
	}

	return ethernets, nil
}

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
