package network

import (
	"github.com/0xef53/kvmrun/server/network"

	pb "github.com/0xef53/kvmrun/api/services/network/v2"
)

func optsFromConfigureRequest(req *pb.ConfigureRequest) *network.HostNetworkOptions {
	opts := network.HostNetworkOptions{}

	switch v := req.Attrs.(type) {
	case *pb.ConfigureRequest_Vlan:
		opts.Attrs = &network.HostNetworAttrs_VLAN{
			VlanID:       v.Vlan.VlanID,
			MTU:          uint16(v.Vlan.MTU),
			ParentIfname: v.Vlan.ParentInterface,
		}
	case *pb.ConfigureRequest_Vxlan:
		opts.Attrs = &network.HostNetworAttrs_VxLAN{
			VNI:        v.Vxlan.VNI,
			MTU:        uint16(v.Vxlan.MTU),
			BindIfname: v.Vxlan.BindInterface,
		}
	case *pb.ConfigureRequest_Router:
		opts.Attrs = &network.HostNetworAttrs_Router{
			Addrs:          v.Router.Addrs,
			MTU:            uint16(v.Router.MTU),
			BindIfname:     v.Router.BindInterface,
			DefaultGateway: v.Router.DefaultGateway,
			InLimit:        v.Router.InLimit,
			OutLimit:       v.Router.OutLimit,
			ProcessID:      v.Router.ProcessID,
		}
	case *pb.ConfigureRequest_Bridge:
		opts.Attrs = &network.HostNetworAttrs_Bridge{
			BridgeIfname: v.Bridge.Ifname,
			MTU:          uint16(v.Bridge.MTU),
		}
	}

	opts.WithSecondStage = req.SecondStage

	return &opts
}

func optsFromDeconfigureRequest(req *pb.DeconfigureRequest) *network.HostNetworkOptions {
	opts := network.HostNetworkOptions{}

	switch v := req.Attrs.(type) {
	case *pb.DeconfigureRequest_Vlan:
		opts.Attrs = &network.HostNetworAttrs_VLAN{
			VlanID: v.Vlan.VlanID,
		}
	case *pb.DeconfigureRequest_Vxlan:
		opts.Attrs = &network.HostNetworAttrs_VxLAN{
			VNI: v.Vxlan.VNI,
		}
	case *pb.DeconfigureRequest_Router:
		opts.Attrs = &network.HostNetworAttrs_Router{
			BindIfname: v.Router.BindInterface,
		}
	case *pb.DeconfigureRequest_Bridge:
		opts.Attrs = &network.HostNetworAttrs_Bridge{
			BridgeIfname: v.Bridge.Ifname,
		}
	}

	return &opts
}
