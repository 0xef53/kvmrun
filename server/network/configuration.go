package network

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/0xef53/kvmrun/internal/hostnet"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/server"

	log "github.com/sirupsen/logrus"
)

func (s *Server) CreateConf(ctx context.Context, vmname, ifname string, opts NetworkSchemeAttrs, configure bool) error {
	ifname = strings.TrimSpace(ifname)

	if err := kvmrun.ValidateLinkName(ifname); err != nil {
		return err
	}

	if opts == nil {
		return fmt.Errorf("empty network scheme opts")
	} else {
		if err := opts.Validate(true); err != nil {
			return err
		}
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname, ifname+"/hostnet"), true, nil, func(l *log.Entry) error {
		schemes, err := GetNetworkSchemes(vmname)
		if err != nil {
			return err
		}

		for _, scheme := range schemes {
			if scheme.Ifname == ifname {
				return fmt.Errorf("%w: ifname = %s", kvmrun.ErrAlreadyExists, ifname)
			}
		}

		schemes = append(schemes, opts.Properties())

		if err := WriteNetworkSchemes(vmname, schemes...); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("cannot create network scheme: %w", err)
	}

	if configure {
		return s.ConfigureHostNetwork(ctx, vmname, ifname, ConfigureStage_ALL)
	}

	return nil
}

func (s *Server) UpdateConf(ctx context.Context, vmname, ifname string, apply bool, updates ...*NetworkSchemeUpdate) error {
	ifname = strings.TrimSpace(ifname)

	if err := kvmrun.ValidateLinkName(ifname); err != nil {
		return err
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname, ifname+"/hostnet"), true, nil, func(l *log.Entry) error {
		schemes, err := GetNetworkSchemes(vmname)
		if err != nil {
			return err
		}

		var scheme *SchemeProperties

		for _, sc := range schemes {
			if sc.Ifname == ifname {
				scheme = sc

				break
			}
		}

		if scheme == nil {
			return fmt.Errorf("%w: ifname = %s", kvmrun.ErrNotFound, ifname)
		}

		for _, update := range updates {
			switch p := update.Property; p {
			case SchemeUpdate_IN_LIMIT, SchemeUpdate_OUT_LIMIT:
				scheme.Set(p.String(), update.Value)
			case SchemeUpdate_MTU:
				scheme.Set(p.String(), update.Value)
			case SchemeUpdate_ADDRS:
				scheme.Set(p.String(), update.Value)
			case SchemeUpdate_GATEWAY4, SchemeUpdate_GATEWAY6:
				scheme.Set(p.String(), update.Value)
			}
		}

		if err := WriteNetworkSchemes(vmname, schemes...); err != nil {
			return err
		}

		if apply {
			if scheme.SchemeType == Scheme_ROUTED {
				attrs, err := scheme.ExtractAttrs_Routed()
				if err != nil {
					return err
				}

				for _, update := range updates {
					switch p := update.Property; p {
					case SchemeUpdate_IN_LIMIT:
						err = hostnet.RouterSetInboundLimits(ifname, attrs.InLimit)
					case SchemeUpdate_OUT_LIMIT:
						err = hostnet.RouterSetOutboundLimits(ifname, attrs.OutLimit, attrs.BindInterface)
					case SchemeUpdate_ADDRS:
						err = func() error {
							if addrUpdates, ok := update.Value.([]*AddrUpdate); ok {
								toAppend, toRemove, err := SplitAddrUpdate(addrUpdates...)
								if err != nil {
									return fmt.Errorf("cannot parse list of IPs updates: %w", err)
								}

								for _, ipnet := range toAppend {
									var unmanaged bool

									// Skip QoS configuring for prefixes from unmanaged networks
									if s.AppConf.VirtNet.UnmanagedNets.Contains(ipnet.IP) {
										unmanaged = true

										l.Infof("Skip QoS configuring for unmanaged %s", ipnet.String())
									}

									err := hostnet.RouterConfigureAddrs(attrs.BindInterface, ifname, unmanaged, ipnet.String())
									if err != nil {
										return err
									}
								}

								for _, ipnet := range toRemove {
									var unmanaged bool

									// Skip QoS deconfiguring for prefixes from unmanaged networks
									if s.AppConf.VirtNet.UnmanagedNets.Contains(ipnet.IP) {
										unmanaged = true

										l.Infof("Skip QoS deconfiguring for unmanaged %s", ipnet.String())
									}

									err := hostnet.RouterDeconfigureAddrs(attrs.BindInterface, ifname, unmanaged, ipnet.String())
									if err != nil {
										return err
									}
								}
							}

							// Send Gratuitous ARP for all router gateways
							return hostnet.RouterAnnounceGateways(attrs.BindInterface, attrs.Addrs, attrs.Gateway4)
						}()
					}

					if err != nil {
						return err
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("cannot update network scheme: %w", err)
	}

	return nil
}

func (s *Server) RemoveConf(ctx context.Context, vmname, ifname string, deconfigure bool) error {
	ifname = strings.TrimSpace(ifname)

	if err := kvmrun.ValidateLinkName(ifname); err != nil {
		return err
	}

	if deconfigure {
		if err := s.DeconfigureHostNetwork(ctx, vmname, ifname); err != nil {
			return err
		}
	}

	err := s.TaskRunFunc(ctx, server.BlockAnyOperations(vmname, ifname+"/hostnet"), true, nil, func(l *log.Entry) error {
		schemes, err := GetNetworkSchemes(vmname)
		if err != nil {
			return err
		}

		origCount := len(schemes)

		schemes = slices.DeleteFunc(schemes, func(scheme *SchemeProperties) bool {
			return scheme.Ifname == ifname
		})

		if origCount != len(schemes) {
			if err := WriteNetworkSchemes(vmname, schemes...); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("cannot remove network scheme: %w", err)
	}

	return nil
}
