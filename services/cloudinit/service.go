package cloudinit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/cloudinit/v1"
	"github.com/0xef53/kvmrun/internal/cloudinit"
	"github.com/0xef53/kvmrun/internal/network"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/services"

	grpc "google.golang.org/grpc"
	grpc_codes "google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
)

var _ pb.CloudInitServiceServer = &ServiceServer{}

func init() {
	services.Register(&ServiceServer{})
}

type ServiceServer struct {
	*services.ServiceServer
}

func (s *ServiceServer) Init(inner *services.ServiceServer) {
	s.ServiceServer = inner
}

func (s *ServiceServer) Name() string {
	return fmt.Sprintf("%T", s)
}

func (s *ServiceServer) Register(server *grpc.Server) {
	pb.RegisterCloudInitServiceServer(server, s)
}

func (s *ServiceServer) BuildImage(ctx context.Context, req *pb.BuildImageRequest) (*pb.BuildImageResponse, error) {
	vmconfdir := filepath.Join(kvmrun.CONFDIR, req.MachineName)

	if _, err := os.Stat(vmconfdir); err != nil {
		if os.IsNotExist(err) {
			return nil, grpc_status.Errorf(grpc_codes.NotFound, req.MachineName)
		}
		return nil, err
	}

	if len(strings.TrimSpace(req.OutputFile)) == 0 {
		req.OutputFile = filepath.Join(kvmrun.CONFDIR, req.MachineName, "config_cidata")
	}

	data := cloudinit.Data{
		Meta: cloudinit.MetadataConfig{
			DSMode:           "local",
			InstanceID:       fmt.Sprintf("i-%s", req.MachineName),
			LocalHostname:    strings.ReplaceAll(req.MachineName, "_", "-"),
			Platform:         strings.TrimSpace(req.Platform),
			Subplatform:      strings.TrimSpace(req.Subplatform),
			Cloudname:        strings.TrimSpace(req.Cloudname),
			Region:           strings.TrimSpace(req.Region),
			AvailabilityZone: strings.TrimSpace(req.AvailabilityZone),
		},
		Network: cloudinit.NetworkConfig{
			Version: 2,
		},
		Hostname: req.Hostname,
		Domain:   req.Domain,
		Timezone: req.Timezone,
	}

	if len(req.VendorConfig) > 0 {
		if err := yaml.Unmarshal(req.VendorConfig, &data.Vendor); err != nil {
			return nil, err
		}
	}

	if len(req.UserConfig) > 0 {
		if err := yaml.Unmarshal(req.UserConfig, &data.User); err != nil {
			return nil, err
		}
	}

	if v, err := buildEthernetsConfig(vmconfdir); err == nil {
		data.Network.Ethernets = v
	} else {
		return nil, err
	}

	if err := cloudinit.GenImage(&data, req.OutputFile); err != nil {
		return nil, err
	}

	return &pb.BuildImageResponse{OutputFile: req.OutputFile}, nil
}

func buildEthernetsConfig(vmconfdir string) (map[string]cloudinit.EthernetConfig, error) {
	macaddrs, err := func() (map[string]string, error) {
		tmp := struct {
			Network []struct {
				Name   string `json:"ifname"`
				Hwaddr string `json:"hwaddr"`
			} `json:"network"`
		}{}

		b, err := os.ReadFile(filepath.Join(vmconfdir, "config"))
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
			Addrs    []string `json:"ips"`
			Gateway4 string   `json:"gateway4"`
			Gateway6 string   `json:"gateway6"`

			// Deprecated
			DEPRECATED_DefaultGateway string `json:"default_gateway"`
		}{}

		b, err := os.ReadFile(filepath.Join(vmconfdir, "config_network"))
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
				ipnet, err := network.ParseIPNet(ipstr)
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
