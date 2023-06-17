package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/network/v1"
)

type vxlanSchemeOptions struct {
	VNI           uint32 `json:"vni"`
	MTU           uint32 `json:"mtu"`
	BindInterface string `json:"bind_interface"`
}

type vxlanScheme struct {
	linkname string
	opts     *vxlanSchemeOptions
}

func (sc *vxlanScheme) Configure(client pb.NetworkServiceClient, secondStage bool) error {
	req := pb.ConfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb.ConfigureRequest_Vxlan{
			Vxlan: &pb.ConfigureRequest_VxlanAttrs{
				VNI:           sc.opts.VNI,
				MTU:           sc.opts.MTU,
				BindInterface: sc.opts.BindInterface,
			},
		},
		SecondStage: secondStage,
	}

	if _, err := client.Configure(context.Background(), &req); err != nil {
		return err
	}

	Info.Printf("successfully configured: %s, vni=%d, mtu=%d\n", sc.linkname, sc.opts.VNI, sc.opts.MTU)

	return nil
}

func (sc *vxlanScheme) Deconfigure(client pb.NetworkServiceClient) error {
	req := pb.DeconfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb.DeconfigureRequest_Vxlan{
			Vxlan: &pb.DeconfigureRequest_VxlanAttrs{
				VNI: sc.opts.VNI,
			},
		},
	}

	if _, err := client.Deconfigure(context.Background(), &req); err != nil {
		return err
	}

	Info.Printf("successfully deconfigured: %s, vni=%d\n", sc.linkname, sc.opts.VNI)

	return nil
}
