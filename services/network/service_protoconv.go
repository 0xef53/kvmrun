package network

import (
	"github.com/0xef53/kvmrun/server/network"

	pb "github.com/0xef53/kvmrun/api/services/network/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"
)

func attrsFromNetworkSchemeOpts(opts *pb_types.NetworkSchemeOpts) network.NetworkSchemeAttrs {
	switch v := opts.Attrs.(type) {
	case *pb_types.NetworkSchemeOpts_Vlan:
		attrs := network.NetworkSchemeAttrs_VLAN{
			VlanID:          v.Vlan.VlanID,
			ParentInterface: v.Vlan.ParentInterface,
		}
		attrs.Ifname = opts.Ifname
		attrs.MTU = opts.MTU
		attrs.Addrs = opts.Addrs
		attrs.Gateway4 = opts.Gateway4
		attrs.Gateway6 = opts.Gateway6

		return &attrs
	case *pb_types.NetworkSchemeOpts_Vxlan:
		attrs := network.NetworkSchemeAttrs_VxLAN{
			VNI:           v.Vxlan.VNI,
			BindInterface: v.Vxlan.BindInterface,
		}
		attrs.Ifname = opts.Ifname
		attrs.MTU = opts.MTU
		attrs.Addrs = opts.Addrs
		attrs.Gateway4 = opts.Gateway4
		attrs.Gateway6 = opts.Gateway6

		return &attrs
	case *pb_types.NetworkSchemeOpts_Router:
		attrs := network.NetworkSchemeAttrs_Routed{
			BindInterface: v.Router.BindInterface,
			InLimit:       v.Router.InLimit,
			OutLimit:      v.Router.OutLimit,
		}
		attrs.Ifname = opts.Ifname
		attrs.MTU = opts.MTU
		attrs.Addrs = opts.Addrs
		attrs.Gateway4 = opts.Gateway4
		attrs.Gateway6 = opts.Gateway6

		return &attrs
	case *pb_types.NetworkSchemeOpts_Bridge:
		attrs := network.NetworkSchemeAttrs_Bridge{
			BridgeInterface: v.Bridge.BridgeName,
		}
		attrs.Ifname = opts.Ifname
		attrs.MTU = opts.MTU
		attrs.Addrs = opts.Addrs
		attrs.Gateway4 = opts.Gateway4
		attrs.Gateway6 = opts.Gateway6

		return &attrs
	}

	attrs := network.NetworkSchemeAttrs_Manual{}

	attrs.Ifname = opts.Ifname
	attrs.MTU = opts.MTU
	attrs.Addrs = opts.Addrs
	attrs.Gateway4 = opts.Gateway4
	attrs.Gateway6 = opts.Gateway6

	return &attrs
}

func setFromUpdateConfRequest(req *pb.UpdateConfRequest) []*network.NetworkSchemeUpdate {
	updates := make([]*network.NetworkSchemeUpdate, 0, 7)

	if req.InLimit != nil {
		updates = append(updates, &network.NetworkSchemeUpdate{
			Property: network.SchemeUpdate_IN_LIMIT,
			Value:    req.InLimit.Value,
		})
	}

	if req.OutLimit != nil {
		updates = append(updates, &network.NetworkSchemeUpdate{
			Property: network.SchemeUpdate_OUT_LIMIT,
			Value:    req.OutLimit.Value,
		})
	}

	if req.MTU != nil {
		updates = append(updates, &network.NetworkSchemeUpdate{
			Property: network.SchemeUpdate_MTU,
			Value:    req.MTU.Value,
		})
	}

	if req.Gateway4 != nil {
		updates = append(updates, &network.NetworkSchemeUpdate{
			Property: network.SchemeUpdate_GATEWAY4,
			Value:    req.Gateway4.Value,
		})
	}

	if req.Gateway6 != nil {
		updates = append(updates, &network.NetworkSchemeUpdate{
			Property: network.SchemeUpdate_GATEWAY6,
			Value:    req.Gateway6.Value,
		})
	}

	if len(req.Addrs) > 0 {
		addrUpdates := make([]*network.AddrUpdate, 0, len(req.Addrs))

		for _, v := range req.Addrs {
			addrUpdates = append(addrUpdates, &network.AddrUpdate{
				Action: network.AddrUpdateAction(v.Action),
				Prefix: v.Addr,
			})
		}

		updates = append(updates, &network.NetworkSchemeUpdate{
			Property: network.SchemeUpdate_ADDRS,
			Value:    addrUpdates,
		})
	}

	return updates
}
