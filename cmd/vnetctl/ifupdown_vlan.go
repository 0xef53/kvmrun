package main

import (
	"context"

	pb "github.com/0xef53/kvmrun/api/services/network/v1"
)

type vlanSchemeOptions struct {
	VlanID uint32 `json:"vlan_id"`
	MTU    uint32 `json:"mtu"`
}

type vlanScheme struct {
	linkname string
	opts     *vlanSchemeOptions
}

func (sc *vlanScheme) Configure(client pb.NetworkServiceClient) error {
	req := pb.ConfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb.ConfigureRequest_Vlan{
			Vlan: &pb.ConfigureRequest_VlanAttrs{
				VlanID: sc.opts.VlanID,
				MTU:    sc.opts.MTU,
			},
		},
	}

	if _, err := client.Configure(context.Background(), &req); err != nil {
		return err
	}

	Info.Printf("successfully configured: %s, vlan_id=%d, mtu=%d\n", sc.linkname, sc.opts.VlanID, sc.opts.MTU)

	return nil
}

func (sc *vlanScheme) Deconfigure(client pb.NetworkServiceClient) error {
	req := pb.DeconfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb.DeconfigureRequest_Vlan{
			Vlan: &pb.DeconfigureRequest_VlanAttrs{
				VlanID: sc.opts.VlanID,
			},
		},
	}

	if _, err := client.Deconfigure(context.Background(), &req); err != nil {
		return err
	}

	Info.Printf("successfully deconfigured: %s, vlan_id=%d\n", sc.linkname, sc.opts.VlanID)

	return nil
}
