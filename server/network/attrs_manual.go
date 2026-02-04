package network

type NetworkSchemeAttrs_Manual struct {
	commonAttrs
}

func (x *NetworkSchemeAttrs_Manual) Validate(strict bool) error {
	return x.commonAttrs.Validate(strict)
}

func (x *NetworkSchemeAttrs_Manual) Properties() *SchemeProperties {
	p := x.commonAttrs.Properties()

	p.SchemeType = Scheme_MANUAL

	return p
}
