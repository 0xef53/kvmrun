package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	qmp "github.com/0xef53/go-qmp"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	qt "github.com/0xef53/kvmrun/pkg/qemu/types"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (x *RPC) StartNBDServer(r *http.Request, args *rpccommon.InstanceRequest, port *int) error {
	var data *rpccommon.NBDParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	(*port) = kvmrun.NBDPORT + args.VM.C.Uid()

	opts := struct {
		Addr qt.InetSocketAddressLegacy `json:"addr"`
	}{
		Addr: qt.InetSocketAddressLegacy{
			Type: "inet",
			Data: qt.InetSocketAddressBase{
				Host: data.ListenAddr,
				Port: strconv.Itoa(*port),
			},
		},
	}

	if err := QPool.Run(args.Name, qmp.Command{"nbd-server-start", &opts}, nil); err != nil {
		return err
	}

	for _, diskPath := range data.Disks {
		d, err := kvmrun.NewDisk(diskPath)
		if err != nil {
			return err
		}
		opts := struct {
			Device   string `json:"device"`
			Writable bool   `json:"writable"`
		}{
			Device:   d.BaseName(),
			Writable: true,
		}
		if err := QPool.Run(args.Name, qmp.Command{"nbd-server-add", &opts}, nil); err != nil {
			return err
		}
	}

	return nil
}

func (x *RPC) StopNBDServer(r *http.Request, args *rpccommon.InstanceRequest, resp *struct{}) error {
	return QPool.Run(args.Name, qmp.Command{"nbd-server-stop", nil}, nil)
}
