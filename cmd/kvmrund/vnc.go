package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
	rpccommon "github.com/0xef53/kvmrun/pkg/rpc/common"
)

func (x *RPC) GetVNCRequisites(r *http.Request, args *rpccommon.InstanceRequest, resp *rpccommon.VNCRequisites) error {
	var data *rpccommon.VNCParams

	if err := json.Unmarshal(args.DataRaw, &data); err != nil {
		return err
	}

	if args.VM.R == nil {
		return &kvmrun.NotRunningError{args.Name}
	}

	if len(data.Password) == 0 {
		p, err := genPassword()
		if err != nil {
			return err
		}
		data.Password = p
	}

	if err := args.VM.R.SetVNCPassword(data.Password); err != nil {
		return err
	}

	resp.Password = data.Password
	resp.Display = args.VM.R.Uid()
	resp.Port = args.VM.R.Uid() + 5900
	resp.WSPort = args.VM.R.Uid() + kvmrun.WEBSOCKSPORT

	return nil
}

func genPassword() (string, error) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5.Sum(buf)), nil
}
