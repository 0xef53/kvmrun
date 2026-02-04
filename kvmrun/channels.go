package kvmrun

import (
	"encoding/json"
	"fmt"
)

type ChannelVSockProperties struct {
	ContextID uint32 `json:"context_id,omitempty"`
}

func (p *ChannelVSockProperties) Validate(_ bool) error {
	if p.ContextID > 0 && p.ContextID < 3 {
		return fmt.Errorf("incorrect context ID (allowed range is from 3 to 0xffffffff - 1)")
	}

	return nil
}

type ChannelVSock struct {
	ChannelVSockProperties

	QemuAddr string `json:"addr,omitempty"`
}

func NewChannelVSock(contextID uint32) (*ChannelVSock, error) {
	dev := new(ChannelVSock)

	dev.ContextID = contextID

	if err := dev.Validate(false); err != nil {
		return nil, err
	}

	return dev, nil
}

func (d *ChannelVSock) Copy() *ChannelVSock {
	v := ChannelVSock{ChannelVSockProperties: d.ChannelVSockProperties}

	v.QemuAddr = d.QemuAddr

	return &v
}

func (d *ChannelVSock) UnmarshalJSON(data []byte) (err error) {
	tmp := struct {
		ChannelVSockProperties

		QemuAddr string `json:"addr,omitempty"`
	}{}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	d.ContextID = tmp.ContextID
	d.QemuAddr = tmp.QemuAddr

	return nil
}
