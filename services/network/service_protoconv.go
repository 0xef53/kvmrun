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

func schemePropertiesToProto(sc *network.SchemeProperties) (*pb_types.NetworkSchemeOpts, error) {
	if sc == nil {
		return nil, nil
	}

	proto := pb_types.NetworkSchemeOpts{
		Ifname: sc.Ifname,
	}

	switch sc.SchemeType {
	case network.Scheme_VLAN:
		attrs, err := sc.ExtractAttrs_VLAN()
		if err != nil {
			return nil, err
		}

		proto.MTU = attrs.MTU
		proto.Addrs = attrs.Addrs
		proto.Gateway4 = attrs.Gateway4
		proto.Gateway6 = attrs.Gateway6

		proto.Attrs = &pb_types.NetworkSchemeOpts_Vlan{
			Vlan: &pb_types.NetworkSchemeOpts_Attrs_VLAN{
				ParentInterface: attrs.ParentInterface,
				VlanID:          attrs.VlanID,
			},
		}
	case network.Scheme_VXLAN:
		attrs, err := sc.ExtractAttrs_VxLAN()
		if err != nil {
			return nil, err
		}

		proto.MTU = attrs.MTU
		proto.Addrs = attrs.Addrs
		proto.Gateway4 = attrs.Gateway4
		proto.Gateway6 = attrs.Gateway6

		proto.Attrs = &pb_types.NetworkSchemeOpts_Vxlan{
			Vxlan: &pb_types.NetworkSchemeOpts_Attrs_VxLAN{
				BindInterface: attrs.BindInterface,
				VNI:           attrs.VNI,
			},
		}
	case network.Scheme_ROUTED:
		attrs, err := sc.ExtractAttrs_Routed()
		if err != nil {
			return nil, err
		}

		proto.MTU = attrs.MTU
		proto.Addrs = attrs.Addrs
		proto.Gateway4 = attrs.Gateway4
		proto.Gateway6 = attrs.Gateway6

		proto.Attrs = &pb_types.NetworkSchemeOpts_Router{
			Router: &pb_types.NetworkSchemeOpts_Attrs_Router{
				BindInterface: attrs.BindInterface,
				InLimit:       attrs.InLimit,
				OutLimit:      attrs.OutLimit,
			},
		}
	case network.Scheme_BRIDGE:
		attrs, err := sc.ExtractAttrs_Bridge()
		if err != nil {
			return nil, err
		}

		proto.MTU = attrs.MTU
		proto.Addrs = attrs.Addrs
		proto.Gateway4 = attrs.Gateway4
		proto.Gateway6 = attrs.Gateway6

		proto.Attrs = &pb_types.NetworkSchemeOpts_Bridge{
			Bridge: &pb_types.NetworkSchemeOpts_Attrs_Bridge{
				BridgeName: attrs.BridgeInterface,
			},
		}
	case network.Scheme_MANUAL:
		attrs, err := sc.ExtractAttrs_COMMON()
		if err != nil {
			return nil, err
		}

		proto.MTU = attrs.MTU
		proto.Addrs = attrs.Addrs
		proto.Gateway4 = attrs.Gateway4
		proto.Gateway6 = attrs.Gateway6
	}

	return &proto, nil
}

func schemesToProto(schemes []*network.SchemeProperties) ([]*pb_types.NetworkSchemeOpts, error) {
	protos := make([]*pb_types.NetworkSchemeOpts, 0, len(schemes))

	for _, sc := range schemes {
		proto, err := schemePropertiesToProto(sc)
		if err != nil {
			return nil, err
		}

		if proto != nil {
			protos = append(protos, proto)
		}
	}

	return protos, nil
}
