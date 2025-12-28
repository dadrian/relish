package relish

import "testing"

func TestTypeIDs(t *testing.T) {
	// Sanity check: ensure values match SPEC.md
	want := []TypeID{
		TypeNull, TypeBool, TypeU8, TypeU16, TypeU32, TypeU64, TypeU128,
		TypeI8, TypeI16, TypeI32, TypeI64, TypeI128,
		TypeF32, TypeF64, TypeString, TypeArray, TypeMap, TypeStruct, TypeEnum, TypeTimestamp,
	}
	got := []TypeID{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13}
	if len(want) != len(got) {
		t.Fatalf("mismatch lengths: %d vs %d", len(want), len(got))
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("TypeID mismatch at %d: got %02x want %02x", i, byte(want[i]), byte(got[i]))
		}
	}
}
