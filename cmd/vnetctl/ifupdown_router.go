package main

import (
	"context"

	pb_network "github.com/0xef53/kvmrun/api/services/network/v2"
)

type routerSchemeOptions struct {
	Addrs          []string `json:"ips"`
	MTU            uint32   `json:"mtu"`
	BindInterface  string   `json:"bind_interface"`
	DefaultGateway string   `json:"default_gateway"`
	InLimit        uint32   `json:"bwlim_in"`
	OutLimit       uint32   `json:"bwlim_out"`
	ProcessID      uint32
}

type routerScheme struct {
	linkname string
	opts     *routerSchemeOptions
}

func (sc *routerScheme) Configure(client pb_network.NetworkServiceClient, secondStage bool) error {
	req := pb_network.ConfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb_network.ConfigureRequest_Router{
			Router: &pb_network.ConfigureRequest_RouterAttrs{
				Addrs:          sc.opts.Addrs,
				MTU:            sc.opts.MTU,
				BindInterface:  sc.opts.BindInterface,
				DefaultGateway: sc.opts.DefaultGateway,
				InLimit:        sc.opts.InLimit,
				OutLimit:       sc.opts.OutLimit,
				ProcessID:      sc.opts.ProcessID,
			},
		},
		SecondStage: secondStage,
	}

	if _, err := client.Configure(context.Background(), &req); err != nil {
		return err
	}

	Info.Printf("successfully configured: %s, mtu=%d\n", sc.linkname, sc.opts.MTU)

	return nil
}

func (sc *routerScheme) Deconfigure(client pb_network.NetworkServiceClient) error {
	req := pb_network.DeconfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb_network.DeconfigureRequest_Router{
			Router: &pb_network.DeconfigureRequest_RouterAttrs{
				BindInterface: sc.opts.BindInterface,
			},
		},
	}

	if _, err := client.Deconfigure(context.Background(), &req); err != nil {
		return err
	}

	Info.Printf("successfully deconfigured: %s\n", sc.linkname)

	return nil
}
