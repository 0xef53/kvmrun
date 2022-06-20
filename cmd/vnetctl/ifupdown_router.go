package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/network/v1"
)

type routerSchemeOptions struct {
	MTU uint32 `json:"mtu"`
}

type routerScheme struct {
	linkname string
	opts     *routerSchemeOptions
}

func (sc *routerScheme) Configure(client pb.NetworkServiceClient) error {
	req := pb.ConfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb.ConfigureRequest_Router{
			Router: &pb.ConfigureRequest_RouterAttrs{
				MTU: sc.opts.MTU,
			},
		},
	}

	if _, err := client.Configure(context.Background(), &req); err != nil {
		return err
	}

	Info.Printf("successfully configured: %s, mtu=%d\n", sc.linkname, sc.opts.MTU)

	return nil
}

func (sc *routerScheme) Deconfigure(client pb.NetworkServiceClient) error {
	req := pb.DeconfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb.DeconfigureRequest_Router{
			Router: &pb.DeconfigureRequest_RouterAttrs{},
		},
	}

	if _, err := client.Deconfigure(context.Background(), &req); err != nil {
		return err
	}

	Info.Printf("successfully deconfigured: %s\n", sc.linkname)

	return nil
}
