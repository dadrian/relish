package internal

import (
	"bytes"
	"testing"
)

func TestFixedSizeAndVarSize(t *testing.T) {
	if n, ok := FixedSize(0x04); !ok || n != 4 { // u32
		t.Fatalf("u32 FixedSize: got (%d,%v) want (4,true)", n, ok)
	}
	if n, ok := FixedSize(0x0E); ok || n != 0 { // string varsize
		t.Fatalf("string FixedSize: got (%d,%v) want (0,false)", n, ok)
	}
	if !IsVarSize(0x0F) || !IsVarSize(0x10) || !IsVarSize(0x11) || !IsVarSize(0x12) {
		t.Fatalf("expected array/map/struct/enum to be varsize")
	}
}

func TestTypeValidation(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteType(&buf, 0x80); err == nil { // top bit set invalid
		t.Fatalf("expected error for invalid type id")
	}
}

func TestU32TLV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteU32TLV(&buf, 42); err != nil {
		t.Fatalf("WriteU32TLV failed: %v", err)
	}
	want := []byte{0x04, 0x2A, 0x00, 0x00, 0x00}
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("encoded mismatch: got %v want %v", got, want)
	}

	// Now read back
	v, err := ReadU32TLV(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadU32TLV failed: %v", err)
	}
	if v != 42 {
		t.Fatalf("decoded mismatch: got %d want 42", v)
	}
}

func TestLengthsEncodeDecode(t *testing.T) {
	// Short form: 0..127
	for _, n := range []int{0, 1, 63, 127} {
		var b [4]byte
		sz := EncodeLen(b[:], n)
		if sz != 1 {
			t.Fatalf("short len size: got %d want 1 (n=%d)", sz, n)
		}
		v, used := DecodeLen(b[:1])
		if used != 1 || v != n {
			t.Fatalf("short decode: got (v=%d,used=%d) want (v=%d,used=1)", v, used, n)
		}
	}
	// Long form boundary
	for _, n := range []int{128, 1024, MaxLen} {
		var b [4]byte
		sz := EncodeLen(b[:], n)
		if sz != 4 {
			t.Fatalf("long len size: got %d want 4 (n=%d)", sz, n)
		}
		v, used := DecodeLen(b[:])
		if used != 4 || v != n {
			t.Fatalf("long decode: got (v=%d,used=%d) want (v=%d,used=4)", v, used, n)
		}
	}
}
