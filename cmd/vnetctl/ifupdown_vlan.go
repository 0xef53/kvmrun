package main

import (
	"context"

	pb_network "github.com/0xef53/kvmrun/api/services/network/v2"
)

type vlanSchemeOptions struct {
	VlanID uint32 `json:"vlan_id"`
	MTU    uint32 `json:"mtu"`
}

type vlanScheme struct {
	linkname string
	opts     *vlanSchemeOptions
}

func (sc *vlanScheme) Configure(client pb_network.NetworkServiceClient, secondStage bool) error {
	req := pb_network.ConfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb_network.ConfigureRequest_Vlan{
			Vlan: &pb_network.ConfigureRequest_VlanAttrs{
				VlanID: sc.opts.VlanID,
				MTU:    sc.opts.MTU,
			},
		},
		SecondStage: secondStage,
	}

	if _, err := client.Configure(context.Background(), &req); err != nil {
		return err
	}

	Info.Printf("successfully configured: %s, vlan_id=%d, mtu=%d\n", sc.linkname, sc.opts.VlanID, sc.opts.MTU)

	return nil
}

func (sc *vlanScheme) Deconfigure(client pb_network.NetworkServiceClient) error {
	req := pb_network.DeconfigureRequest{
		LinkName: sc.linkname,
		Attrs: &pb_network.DeconfigureRequest_Vlan{
			Vlan: &pb_network.DeconfigureRequest_VlanAttrs{
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
