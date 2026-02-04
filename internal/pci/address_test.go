package pci

import (
	"fmt"
	"strconv"
	"testing"
)

func resultStr(value string, want, got error) string {
	return fmt.Sprintf("got unexpected result:\n\tvalue:\t%s\n\twant:\t%v\n\tgot:\t%v", value, want, got)
}

func TestAddressParseHexString(t *testing.T) {
	var err error

	// Valid cases
	for _, s := range []string{":00:00.0", "1:03:00.0", "0001:ff:00.0", "ffff:af:1f.7"} {
		_, err = AddressFromHex(s)
		if err != nil {
			t.Fatal(resultStr(s, nil, err))
		}
	}

	// Invalid format cases
	convErr := &strconv.NumError{Err: fmt.Errorf("error")}

	for _, s := range []string{"z:03:00.0", "qwerty:03:00.0", "0000:03:yy.0", "0000:03:00.nn"} {
		_, err = AddressFromHex(s)
		if _, ok := err.(*strconv.NumError); !ok {
			t.Fatal(resultStr(s, convErr, err))
		}
	}
}

func TestAddressParseDeviceValues(t *testing.T) {
	var err error

	// Invalid values cases
	valueErr := fmt.Errorf("value error")

	for _, s := range []string{"0000:03:2f.0"} {
		_, err = AddressFromHex(s)
		if err == nil {
			t.Fatal(resultStr(s, valueErr, err))
		}
	}
}

func TestAddressParseFunctionValues(t *testing.T) {
	var err error

	// Invalid values cases
	valueErr := fmt.Errorf("value error")

	for _, s := range []string{"0000:03:1f.8"} {
		_, err = AddressFromHex(s)
		if err == nil {
			t.Fatal(resultStr(s, valueErr, err))
		}
	}
}
