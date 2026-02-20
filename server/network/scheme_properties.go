package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"

	"github.com/0xef53/kvmrun/internal/utils"
	"github.com/0xef53/kvmrun/kvmrun"
)

var (
	ErrInterfaceNotFound = errors.New("interface not found")
	ErrAttributeNotFound = errors.New("attribute not found")
)

type SchemeProperties struct {
	sync.Mutex

	SchemeType SchemeType
	Ifname     string

	// all other attributes
	attrs map[string]interface{}
}

func (p *SchemeProperties) Set(key string, value interface{}) error {
	p.Lock()
	defer p.Unlock()

	if p.attrs == nil {
		p.attrs = make(map[string]interface{})
	}

	switch key {
	case "scheme":
		switch v := value.(type) {
		case string:
			p.SchemeType = SchemeTypeValue(v)
		case SchemeType:
			p.SchemeType = v
		default:
			return fmt.Errorf("incompatible scheme value type")
		}
	case "ifname":
		if s, ok := value.(string); ok {
			p.Ifname = s
		} else {
			return fmt.Errorf("not a string value")
		}
	}

	if addrUpdates, ok := value.([]*AddrUpdate); ok && key == "addrs" {
		var origAddrList []string

		if a, found := p.attrs[key]; found {
			if addrs, ok := a.([]string); ok {
				origAddrList = addrs
			}
		}

		newAddrList, err := p.patchAddrs(origAddrList, addrUpdates...)
		if err != nil {
			return fmt.Errorf("failed to patch original 'addrs' property: %w", err)
		}

		p.attrs[key] = newAddrList
	} else {
		p.attrs[key] = value
	}

	return nil
}

func (p *SchemeProperties) patchAddrs(orig []string, updates ...*AddrUpdate) ([]string, error) {
	inMap := make(map[string]struct{})

	for _, addr := range orig {
		ipnet, err := utils.ParseIPNet(addr)
		if err != nil {
			// ignore bad formatted entry
			continue
		}

		inMap[ipnet.String()] = struct{}{}
	}

	for _, update := range updates {
		ipnet, err := utils.ParseIPNet(update.Prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid IP address: %s", update.Prefix)
		}

		switch update.Action {
		case AddrUpdate_APPEND:
			inMap[ipnet.String()] = struct{}{}
		case AddrUpdate_REMOVE:
			delete(inMap, ipnet.String())
		}
	}

	result := make([]string, 0, len(inMap))

	for addr := range inMap {
		result = append(result, addr)
	}

	return result, nil
}

func (p *SchemeProperties) ValueAs(key string, target interface{}) error {
	p.Lock()
	defer p.Unlock()

	return p.valueAs(key, target)
}

func (p *SchemeProperties) valueAs(key string, target interface{}) (err error) {
	if target == nil {
		return fmt.Errorf("target must be a non-nil pointer")
	}

	if p.attrs == nil {
		return fmt.Errorf("%w (ifname = %s): %s", ErrAttributeNotFound, p.Ifname, key)
	}

	if _, ok := p.attrs[key]; !ok {
		return fmt.Errorf("%w (ifname = %s): %s", ErrAttributeNotFound, p.Ifname, key)
	}

	targetRV := reflect.ValueOf(target)

	if targetRV.Kind() != reflect.Ptr || targetRV.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer to a variable")
	}

	targetElem := targetRV.Elem()

	if !targetElem.CanSet() {
		return fmt.Errorf("target is not settable")
	}

	var value interface{}

	switch {
	case key == "scheme":
		value = p.SchemeType
	case key == "ifname" && len(p.Ifname) > 0:
		value = p.Ifname
	default:
		value = p.attrs[key]
	}

	if value == nil {
		return fmt.Errorf("value is nil (no type information): key = %s", key)
	}

	valueRV := reflect.ValueOf(value)

	// Is value from attrs can be converted to the target ?
	if !valueRV.Type().ConvertibleTo(targetElem.Type()) {
		return fmt.Errorf("type mismatch: key = %s, value type = %s, target type = %s", key, valueRV.Type(), targetElem.Type())
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("type mismatch: key = %s, %v", key, r)
		}
	}()
	targetElem.Set(valueRV.Convert(targetElem.Type()))

	return nil
}

func (p *SchemeProperties) UnmarshalJSON(data []byte) error {
	attrs := make(map[string]interface{})

	if err := json.Unmarshal(data, &attrs); err != nil {
		return err
	}

	if a, found := attrs["scheme"]; found {
		switch v := a.(type) {
		case string:
			p.SchemeType = SchemeTypeValue(v)
		case SchemeType:
			p.SchemeType = v
		default:
			return fmt.Errorf("incompatible scheme value type")
		}
	}

	if v, found := attrs["addrs"]; found {
		m := make(map[string]struct{})

		if aa, ok := v.([]interface{}); ok {
			for _, a := range aa {
				if ipstr, ok := a.(string); ok {
					m[ipstr] = struct{}{}
				}
			}
		}

		addrs := make([]string, 0, len(m))

		for ipstr := range m {
			addrs = append(addrs, ipstr)
		}

		attrs["addrs"] = addrs
	}

	p.attrs = attrs

	if err := p.valueAs("ifname", &p.Ifname); err != nil {
		return err
	}

	return nil
}

func (p *SchemeProperties) MarshalJSON() ([]byte, error) {
	if p.attrs == nil {
		p.attrs = make(map[string]interface{})
	}

	p.attrs["scheme"] = p.SchemeType.String()
	p.attrs["ifname"] = p.Ifname

	return json.Marshal(p.attrs)
}

func (p *SchemeProperties) extractCommonAttrs() (*commonAttrs, error) {
	attrs := commonAttrs{
		Ifname: p.Ifname,
	}

	if err := p.valueAs("mtu", &attrs.MTU); err != nil && !errors.Is(err, ErrAttributeNotFound) {
		return nil, err
	}

	if err := p.valueAs("addrs", &attrs.Addrs); err != nil && !errors.Is(err, ErrAttributeNotFound) {
		return nil, err
	}

	if err := p.valueAs("gateway4", &attrs.Gateway4); err != nil && !errors.Is(err, ErrAttributeNotFound) {
		return nil, err
	}

	if err := p.valueAs("gateway6", &attrs.Gateway6); err != nil && !errors.Is(err, ErrAttributeNotFound) {
		return nil, err
	}

	return &attrs, nil
}

func (p *SchemeProperties) ExtractAttrs_Routed() (*NetworkSchemeAttrs_Routed, error) {
	common, err := p.extractCommonAttrs()
	if err != nil {
		return nil, err
	}

	attrs := NetworkSchemeAttrs_Routed{
		commonAttrs: *common,
	}

	if err := p.valueAs("bind_interface", &attrs.BindInterface); err != nil {
		return nil, err
	}

	if err := p.valueAs("in_limit", &attrs.InLimit); err != nil && !errors.Is(err, ErrAttributeNotFound) {
		return nil, err
	}

	if err := p.valueAs("out_limit", &attrs.OutLimit); err != nil && !errors.Is(err, ErrAttributeNotFound) {
		return nil, err
	}

	return &attrs, nil
}

func (p *SchemeProperties) ExtractAttrs_Bridge() (*NetworkSchemeAttrs_Bridge, error) {
	common, err := p.extractCommonAttrs()
	if err != nil {
		return nil, err
	}

	attrs := NetworkSchemeAttrs_Bridge{
		commonAttrs: *common,
	}

	if err := p.valueAs("bridge_interface", &attrs.BridgeInterface); err != nil {
		return nil, err
	}

	return &attrs, nil
}

func (p *SchemeProperties) ExtractAttrs_VxLAN() (*NetworkSchemeAttrs_VxLAN, error) {
	common, err := p.extractCommonAttrs()
	if err != nil {
		return nil, err
	}

	attrs := NetworkSchemeAttrs_VxLAN{
		commonAttrs: *common,
	}

	if err := p.valueAs("bind_interface", &attrs.BindInterface); err != nil {
		return nil, err
	}

	if err := p.valueAs("vni", &attrs.VNI); err != nil {
		return nil, err
	}

	return &attrs, nil
}

func (p *SchemeProperties) ExtractAttrs_VLAN() (*NetworkSchemeAttrs_VLAN, error) {
	common, err := p.extractCommonAttrs()
	if err != nil {
		return nil, err
	}

	attrs := NetworkSchemeAttrs_VLAN{
		commonAttrs: *common,
	}

	if err := p.valueAs("parent_interface", &attrs.ParentInterface); err != nil {
		return nil, err
	}

	if err := p.valueAs("vlan_id", &attrs.VlanID); err != nil {
		return nil, err
	}

	return &attrs, nil
}

func GetNetworkSchemes(vmname string, ifnames ...string) ([]*SchemeProperties, error) {
	// Check if machine exists
	if _, err := kvmrun.GetInstanceConf(vmname); err != nil {
		return nil, err
	}

	config := filepath.Join(kvmrun.CONFDIR, vmname, "config_network")

	schemes := make([]*SchemeProperties, 0, 5)

	if b, err := os.ReadFile(config); err == nil {
		if err := json.Unmarshal(b, &schemes); err != nil {
			return nil, err
		}
	} else {
		if os.IsNotExist(err) {
			// no one found, no problem
			return nil, nil
		}
		return nil, err
	}

	if len(ifnames) == 0 {
		return schemes, nil
	}

	requested := make([]*SchemeProperties, 0, len(ifnames))

	for _, scheme := range schemes {
		if slices.Contains(ifnames, scheme.Ifname) {
			requested = append(requested, scheme)
		}
	}

	return requested, nil
}

func WriteNetworkSchemes(vmname string, schemes ...*SchemeProperties) error {
	// Check if machine exists
	if _, err := kvmrun.GetInstanceConf(vmname); err != nil {
		return err
	}

	config := filepath.Join(kvmrun.CONFDIR, vmname, "config_network")

	b, err := json.MarshalIndent(schemes, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(config, append(b, '\n'), 0644)
}

type SchemeUpdateProperty uint16

const (
	SchemeUpdate_UNKNOWN SchemeUpdateProperty = iota
	SchemeUpdate_IN_LIMIT
	SchemeUpdate_OUT_LIMIT
	SchemeUpdate_MTU
	SchemeUpdate_ADDRS
	SchemeUpdate_GATEWAY4
	SchemeUpdate_GATEWAY6
)

func (p SchemeUpdateProperty) String() string {
	switch p {
	case SchemeUpdate_IN_LIMIT:
		return "in_limit"
	case SchemeUpdate_OUT_LIMIT:
		return "out_limit"
	case SchemeUpdate_MTU:
		return "mtu"
	case SchemeUpdate_ADDRS:
		return "addrs"
	case SchemeUpdate_GATEWAY4:
		return "gateway4"
	case SchemeUpdate_GATEWAY6:
		return "gateway6"
	}

	return "UNKNOWN"
}

type NetworkSchemeUpdate struct {
	Property SchemeUpdateProperty
	Value    interface{}
}

type AddrUpdateAction uint16

const (
	AddrUpdate_UNKNOWN AddrUpdateAction = iota
	AddrUpdate_APPEND
	AddrUpdate_REMOVE
)

type AddrUpdate struct {
	Action AddrUpdateAction
	Prefix string
}
