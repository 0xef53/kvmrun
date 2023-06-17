package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/network/v1"
)

type bridgeSchemeOptions struct {
	BrName string `json:"brname"`
	MTU    uint32 `json:"mtu"`
}

type bridgeScheme struct {
	linkname string
	opts     *bridgeSchemeOptions
}

func (sc *bridgeScheme) Configure(client pb.NetworkServiceClient, secondStage bool) error {
	req := pb.ConfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb.ConfigureRequest_Bridge{
			Bridge: &pb.ConfigureRequest_BridgeAttrs{
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

func (sc *bridgeScheme) Deconfigure(client pb.NetworkServiceClient) error {
	req := pb.DeconfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb.DeconfigureRequest_Bridge{
			Bridge: &pb.DeconfigureRequest_BridgeAttrs{
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
