package version

import (
	"errors"
	"strconv"
	"testing"
)

func TestParseFromString(t *testing.T) {
	goodValues := map[string]string{
		" 1.2.3": "1.2.3",
		"2.11.1": "2.11.1",
		"0.11.1": "0.11.1",
		"0.0.6 ": "0.0.6",
		"  0.9 ": "0.9.0",
		"  35  ": "35.0.0",
		" 2397 ": "2397.0.0",
		"     0": "0.0.0",
	}

	for orig, want := range goodValues {
		got, err := Parse(orig)
		if err != nil {
			t.Fatalf("got unexpected error (orig value = '%q'):\nerror:\t%v", orig, err)
		}
		if got.String() != want {
			t.Fatalf("got invalid result (orig value = '%q'):\nwant:\t%q\ngot:\t%q", orig, want, got)
		}
	}

	// bad values tests

	badValue1 := "3.4.5.6"

	if _, err := Parse(badValue1); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("got unexpected error (orig value = '%q'):\nwant error:\tErrInvalidValue\ngot error:\t%v", badValue1, err)
	}

	badValue2 := "3.a.5"

	if _, err := Parse(badValue2); true {
		if _, ok := err.(*strconv.NumError); !ok {
			t.Fatalf("got unexpected error (orig value = '%q'):\nwant error:\tstrconv.NumError\ngot error:\t%v", badValue2, err)
		}
	}
}

func TestParseFromInt(t *testing.T) {
	goodValues := map[int]string{
		10203:    "1.2.3",
		21101:    "2.11.1",
		1101:     "0.11.1",
		6:        "0.0.6",
		900:      "0.9.0",
		350000:   "35.0.0",
		23970000: "2397.0.0",
		0:        "0.0.0",
	}

	for orig, want := range goodValues {
		got, err := Parse(orig)
		if err != nil {
			t.Fatalf("got unexpected error:\norig value:\t%q\nerror:\t%v", orig, err)
		}
		if got.String() != want {
			t.Fatalf("got invalid result (orig value = %d):\nwant:\t%q\ngot:\t%q", orig, want, got)
		}
	}
}

func TestParseFromUnsupportedValueType(t *testing.T) {
	badValue1 := 36.6

	if _, err := Parse(badValue1); !errors.Is(err, ErrInvalidValueType) {
		t.Fatalf("got unexpected error (orig value = %v):\nwant error:\tErrInvalidValueType\ngot error:\t%v", badValue1, err)
	}
}
