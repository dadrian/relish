package relish

import (
	"bytes"
	"reflect"
	"testing"
)

// assertRoundtrip decodes the provided bytes into a value of the same
// dynamic type as expected, and then re-encodes it. It expects both
// operations to succeed and match the expected structures and bytes.
func assertRoundtrip(t *testing.T, expected any, b []byte) {
	t.Helper()

	// Decode
	dstPtr := reflect.New(reflect.TypeOf(expected))
	if err := Unmarshal(b, dstPtr.Interface()); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	got := reflect.Indirect(dstPtr).Interface()
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("decoded value mismatch:\n got: %#v\nwant: %#v", got, expected)
	}

	// Encode
	enc, err := Marshal(expected)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if !bytes.Equal(enc, b) {
		t.Fatalf("encoded bytes mismatch:\n got: %v\nwant: %v", enc, b)
	}
}

func Test_SimpleStruct(t *testing.T) {
	type Simple struct {
		Value uint32 `relish:"0"`
	}
	assertRoundtrip(t, Simple{Value: 42}, []byte{0x11, 0x0C, 0x00, 0x04, 0x2A, 0x00, 0x00, 0x00})
}

func Test_MultipleFields(t *testing.T) {
	type MultiField struct {
		A uint32 `relish:"0"`
		B string `relish:"1"`
		C bool   `relish:"5"`
	}
	assertRoundtrip(t, MultiField{A: 42, B: "hello", C: true}, []byte{
		0x11, 0x22, 0x00, 0x04, 0x2A, 0x00, 0x00, 0x00, 0x01, 0x0E, 0x0A, 'h', 'e', 'l', 'l', 'o', 0x05, 0x01, 0xFF,
	})
}

func ptr[T any](v T) *T { return &v }

func Test_OptionalFields(t *testing.T) {
	type WithOption struct {
		Required uint32  `relish:"0"`
		Optional *uint32 `relish:"1,optional"`
	}
	assertRoundtrip(t, WithOption{Required: 10, Optional: ptr(uint32(20))}, []byte{
		0x11, 0x18, 0x00, 0x04, 0x0A, 0x00, 0x00, 0x00, 0x01, 0x04, 0x14, 0x00, 0x00, 0x00,
	})
	assertRoundtrip(t, WithOption{Required: 10, Optional: nil}, []byte{0x11, 0x0C, 0x00, 0x04, 0x0A, 0x00, 0x00, 0x00})
}

func Test_SkipField(t *testing.T) {
	type WithSkip struct {
		Included uint32 `relish:"0"`
		Skipped  string `relish:"-"`
	}
	// Expect the skipped field not to be serialized; only Included appears.
	// The decoded value should have the zero value for Skipped.
	v := WithSkip{Included: 42, Skipped: ""}
	b := []byte{0x11, 0x0C, 0x00, 0x04, 0x2A, 0x00, 0x00, 0x00}
	assertRoundtrip(t, v, b)
}

func Test_SkipField_PreserveExisting(t *testing.T) {
	type WithSkip struct {
		Included uint32 `relish:"0"`
		Skipped  string `relish:"-"`
	}
	// Initial struct has a non-zero skipped field value.
	v := WithSkip{Skipped: "preserve me"}
	// Bytes only encode the Included field; Skipped is not serialized.
	data := []byte{0x11, 0x0C, 0x00, 0x04, 0x2A, 0x00, 0x00, 0x00}
	if err := Unmarshal(data, &v); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if v.Included != 42 {
		t.Fatalf("Included mismatch: got %v want 42", v.Included)
	}
	if v.Skipped != "preserve me" {
		t.Fatalf("Skipped was modified: got %q want %q", v.Skipped, "preserve me")
	}
}

func Test_EmptyStruct(t *testing.T) {
	type Empty struct{}
	assertRoundtrip(t, Empty{}, []byte{0x11, 0x00})
}

func Test_ParseWithUnknownFields(t *testing.T) {
	type Partial struct {
		A uint32 `relish:"0"`
	}
	data := []byte{0x11, 0x1C, 0x00, 0x04, 0x2A, 0x00, 0x00, 0x00, 0x02, 0x0E, 0x0A, 'h', 'e', 'l', 'l', 'o'}
	var got Partial
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got.A != 42 {
		t.Fatalf("unexpected value: got %v want 42", got.A)
	}
}

func Test_ParseFieldsNotInOrder(t *testing.T) {
	type Ordered struct {
		A uint32 `relish:"0"`
		B uint32 `relish:"1"`
	}
	data := []byte{0x11, 0x18, 0x01, 0x04, 0x14, 0x00, 0x00, 0x00, 0x00, 0x04, 0x0A, 0x00, 0x00, 0x00}
	var got Ordered
	err := Unmarshal(data, &got)
	if e, ok := err.(*Error); !ok || e.Kind != ErrFieldOrder {
		t.Fatalf("expected ErrFieldOrder, got %v", err)
	}
}

func Test_NestedStructs(t *testing.T) {
	type Inner struct {
		Value uint32 `relish:"0"`
	}
	type Outer struct {
		Inner Inner  `relish:"0"`
		Other uint32 `relish:"1"`
	}
	assertRoundtrip(t, Outer{Inner: Inner{Value: 10}, Other: 20}, []byte{
		0x11, 0x1E, 0x00, 0x11, 0x0C, 0x00, 0x04, 0x0A, 0x00, 0x00, 0x00, 0x01, 0x04, 0x14, 0x00, 0x00, 0x00,
	})
}

// Enum-like tests. These rely on eventual enum support; they are expected to fail until implemented.
func Test_SimpleEnum(t *testing.T) {
	type SimpleEnum struct {
		A *uint32 `relish:"0,optional"`
		B *string `relish:"1,optional"`
	}
	assertRoundtrip(t, SimpleEnum{A: ptr(uint32(42))}, []byte{0x12, 0x0C, 0x00, 0x04, 0x2A, 0x00, 0x00, 0x00})
	assertRoundtrip(t, SimpleEnum{B: ptr("hello")}, []byte{0x12, 0x10, 0x01, 0x0E, 0x0A, 'h', 'e', 'l', 'l', 'o'})
}

func Test_EnumWithNestedStruct(t *testing.T) {
	type Inner struct {
		Value uint32 `relish:"0"`
	}
	type EnumWithStruct struct {
		Simple  *uint32 `relish:"0,optional"`
		Complex *Inner  `relish:"1,optional"`
	}
	assertRoundtrip(t, EnumWithStruct{Simple: ptr(uint32(10))}, []byte{0x12, 0x0C, 0x00, 0x04, 0x0A, 0x00, 0x00, 0x00})
	assertRoundtrip(t, EnumWithStruct{Complex: &Inner{Value: 20}}, []byte{0x12, 0x12, 0x01, 0x11, 0x0C, 0x00, 0x04, 0x14, 0x00, 0x00, 0x00})
}

func Test_NestedEnums(t *testing.T) {
	type Inner struct {
		X *uint32 `relish:"0,optional"`
		Y *string `relish:"1,optional"`
	}
	type Outer struct {
		Nested *Inner  `relish:"0,optional"`
		Value  *uint32 `relish:"1,optional"`
	}
	assertRoundtrip(t, Outer{Nested: &Inner{X: ptr(uint32(42))}}, []byte{0x12, 0x12, 0x00, 0x12, 0x0C, 0x00, 0x04, 0x2A, 0x00, 0x00, 0x00})
	assertRoundtrip(t, Outer{Value: ptr(uint32(10))}, []byte{0x12, 0x0C, 0x01, 0x04, 0x0A, 0x00, 0x00, 0x00})
}

func Test_EnumUnknownVariant(t *testing.T) {
	type SimpleEnum struct {
		A *uint32 `relish:"0,optional"`
	}
	data := []byte{0x12, 0x0C, 0x05, 0x04, 0x2A, 0x00, 0x00, 0x00}
	var got SimpleEnum
	if err := Unmarshal(data, &got); err == nil {
		t.Fatalf("expected error for unknown enum variant, got nil")
	}
}

func Test_EnumWithExtraData(t *testing.T) {
	type SimpleEnum struct {
		A *uint32 `relish:"0,optional"`
	}
	// Valid enum value with an extra padding byte at the end.
	data := []byte{0x12, 0x0E, 0x00, 0x04, 0x2A, 0x00, 0x00, 0x00, 0xFF}
	var got SimpleEnum
	if err := Unmarshal(data, &got); err == nil {
		t.Fatalf("expected error due to extra data, got nil")
	}
}
