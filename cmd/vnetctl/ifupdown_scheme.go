package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xef53/kvmrun/kvmrun"

	pb_network "github.com/0xef53/kvmrun/api/services/network/v2"
)

var errSchemeNotFound = errors.New("scheme not found")

type Scheme interface {
	Configure(pb_network.NetworkServiceClient, bool) error
	Deconfigure(pb_network.NetworkServiceClient) error
}

func GetNetworkScheme(linkname string, configs ...string) (Scheme, error) {
	for _, fname := range configs {
		b, err := os.ReadFile(fname)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		netconf := []json.RawMessage{}

		if err := json.Unmarshal(b, &netconf); err != nil {
			return nil, err
		}

		for _, b := range netconf {
			scheme := struct {
				Type   string `json:"scheme"`
				Ifname string `json:"ifname"`
			}{}

			if err := json.Unmarshal(b, &scheme); err != nil {
				return nil, err
			}

			if scheme.Ifname == linkname {
				switch strings.ToLower(scheme.Type) {
				case "vlan", "bridge-vlan":
					opts := vlanSchemeOptions{}

					if err := json.Unmarshal(b, &opts); err != nil {
						return nil, err
					}

					return &vlanScheme{linkname, &opts}, nil

				case "vxlan", "bridge-vxlan":
					opts := vxlanSchemeOptions{}

					if err := json.Unmarshal(b, &opts); err != nil {
						return nil, err
					}

					return &vxlanScheme{linkname, &opts}, nil
				case "routed":
					/*
						PID is needed to configure net_cls.classid for use in traffic control rules.
					*/
					pid, err := func() (uint32, error) {
						if cwd, err := os.Getwd(); err == nil {
							if b, err := os.ReadFile(filepath.Join(kvmrun.CHROOTDIR, filepath.Base(cwd), "pid")); err == nil {
								if v, err := strconv.ParseUint(string(b), 10, 32); err == nil {
									return uint32(v), nil
								} else {
									return 0, err
								}
							} else {
								return 0, err
							}
						} else {
							return 0, err
						}
					}()

					if err != nil {
						return nil, fmt.Errorf("failed to determine the machine process ID: %w", err)
					}

					opts := routerSchemeOptions{ProcessID: pid}

					if err := json.Unmarshal(b, &opts); err != nil {
						return nil, err
					}

					return &routerScheme{linkname, &opts}, nil
				case "bridge":
					opts := bridgeSchemeOptions{}

					if err := json.Unmarshal(b, &opts); err != nil {
						return nil, err
					}

					return &bridgeScheme{linkname, &opts}, nil
				}
			}
		}
	}

	return nil, errSchemeNotFound
}
