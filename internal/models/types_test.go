package models

import (
	"reflect"
	"testing"
)

func TestInt64ListValueScanRoundTrip(t *testing.T) {
	cases := []Int64List{
		nil,
		{},
		{1},
		{1, 2, 3, 42},
	}
	for _, original := range cases {
		v, err := original.Value()
		if err != nil {
			t.Fatalf("Value() error for %v: %v", original, err)
		}
		var scanned Int64List
		if err := scanned.Scan(v); err != nil {
			t.Fatalf("Scan() error for %v: %v", original, err)
		}
		if len(original) == 0 {
			if len(scanned) != 0 {
				t.Errorf("expected empty result for %v, got %v", original, scanned)
			}
			continue
		}
		if !reflect.DeepEqual([]int64(original), []int64(scanned)) {
			t.Errorf("round trip mismatch: original %v, scanned %v", original, scanned)
		}
	}
}

func TestInt64ListScanNil(t *testing.T) {
	var l Int64List
	if err := l.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) error: %v", err)
	}
	if len(l) != 0 {
		t.Errorf("expected empty list, got %v", l)
	}
}

func TestInt64ListScanFromStringAndBytes(t *testing.T) {
	var fromBytes Int64List
	if err := fromBytes.Scan([]byte(`[5,6,7]`)); err != nil {
		t.Fatalf("Scan([]byte) error: %v", err)
	}
	if !reflect.DeepEqual([]int64{5, 6, 7}, []int64(fromBytes)) {
		t.Errorf("Scan([]byte) mismatch: got %v", fromBytes)
	}

	var fromString Int64List
	if err := fromString.Scan(`[8,9]`); err != nil {
		t.Fatalf("Scan(string) error: %v", err)
	}
	if !reflect.DeepEqual([]int64{8, 9}, []int64(fromString)) {
		t.Errorf("Scan(string) mismatch: got %v", fromString)
	}
}

func TestInt64ListScanUnsupportedType(t *testing.T) {
	var l Int64List
	if err := l.Scan(42); err == nil {
		t.Error("expected error scanning unsupported type, got nil")
	}
}
