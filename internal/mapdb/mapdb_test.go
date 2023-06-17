package mapdb

import (
	"testing"
)

func TestBasic(t *testing.T) {
	MIN_VALUE, MAX_VALUE = 5, 12

	if v, err := getVacantValue([]int{}); err == nil {
		if v != MIN_VALUE {
			t.Fatalf("[1] got %d value instead of %d", v, MIN_VALUE)
		}
	} else {
		t.Fatalf("[1] %s", err)
	}

	if v, err := getVacantValue([]int{MIN_VALUE}); err == nil {
		if v != MIN_VALUE+1 {
			t.Fatalf("[2] got %d value instead of %d", v, MIN_VALUE+1)
		}
	} else {
		t.Fatalf("[2] %s", err)
	}

	if v, err := getVacantValue([]int{200, 3, 100, 1}); err == nil {
		if v != MIN_VALUE {
			t.Fatalf("[3] got %d value instead of %d", v, MIN_VALUE)
		}
	} else {
		t.Fatalf("[3] %s", err)
	}

	if v, err := getVacantValue([]int{7, 6, 6, 5, 11, 100, 8, 2, 9, 10, 6, 8}); err == nil {
		if v != MAX_VALUE {
			t.Fatalf("[4] got %d value instead of %d", v, MAX_VALUE)
		}
	} else {
		t.Fatalf("[4] %s", err)
	}

	if v, err := getVacantValue([]int{11, 6, 12, 8, 100, 2, 7, 10}); err == nil {
		if v != MIN_VALUE {
			t.Fatalf("[5] got %d value instead of %d", v, MIN_VALUE)
		}
	} else {
		t.Fatalf("[5] %s", err)
	}

	// Should be full
	if v, err := getVacantValue([]int{12, 8, 6, 2, 5, 9, 400, 10, 7, 11}); err != nil {
		if err != ErrNoAvailableValues {
			t.Fatalf("[6] %s", err)
		}
	} else {
		t.Fatalf("[6] got %d value instead of no-available error", v)
	}
}
