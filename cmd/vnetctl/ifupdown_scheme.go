package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/network/v1"
)

var errSchemeNotFound = errors.New("scheme not found")

type Scheme interface {
	Configure(pb.NetworkServiceClient) error
	Deconfigure(pb.NetworkServiceClient) error
}

func GetNetworkScheme(linkname string, configs ...string) (Scheme, error) {
	for _, fname := range configs {
		b, err := ioutil.ReadFile(fname)
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
					opts := routerSchemeOptions{}

					if err := json.Unmarshal(b, &opts); err != nil {
						return nil, err
					}

					return &routerScheme{linkname, &opts}, nil
				}
			}
		}
	}

	return nil, errSchemeNotFound
}
