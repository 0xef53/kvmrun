package main

import (
	"context"

	pb_network "github.com/0xef53/kvmrun/api/services/network/v2"
)

type bridgeSchemeOptions struct {
	BrName string `json:"brname"`
	MTU    uint32 `json:"mtu"`
}

type bridgeScheme struct {
	linkname string
	opts     *bridgeSchemeOptions
}

func (sc *bridgeScheme) Configure(client pb_network.NetworkServiceClient, secondStage bool) error {
	req := pb_network.ConfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb_network.ConfigureRequest_Bridge{
			Bridge: &pb_network.ConfigureRequest_BridgeAttrs{
				Ifname: sc.opts.BrName,
				MTU:    sc.opts.MTU,
			},
		},
		SecondStage: secondStage,
	}

	if _, err := client.Configure(context.Background(), &req); err != nil {
		return err
	}

	Info.Printf("successfully configured: %s, brname=%s, mtu=%d\n", sc.linkname, sc.opts.BrName, sc.opts.MTU)

	return nil
}

func (sc *bridgeScheme) Deconfigure(client pb_network.NetworkServiceClient) error {
	req := pb_network.DeconfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb_network.DeconfigureRequest_Bridge{
			Bridge: &pb_network.DeconfigureRequest_BridgeAttrs{
				Ifname: sc.opts.BrName,
			},
		},
	}

	if _, err := client.Deconfigure(context.Background(), &req); err != nil {
		return err
	}

	Info.Printf("successfully deconfigured: %s, brname=%s\n", sc.linkname, sc.opts.BrName)

	return nil
}
