package internal

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestNullTLV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteNullTLV(&buf); err != nil {
		t.Fatalf("write null: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x00}) {
		t.Fatalf("got %v", buf.Bytes())
	}
	if err := ReadNullTLV(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("read null: %v", err)
	}
}

func TestBoolTLV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteBoolTLV(&buf, true); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x01, 0xFF}) {
		t.Fatalf("got %v", buf.Bytes())
	}
	v, err := ReadBoolTLV(bytes.NewReader(buf.Bytes()))
	if err != nil || v != true {
		t.Fatalf("read err=%v v=%v", err, v)
	}

	buf.Reset()
	if err := WriteBoolTLV(&buf, false); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x01, 0x00}) {
		t.Fatalf("got %v", buf.Bytes())
	}
	v, err = ReadBoolTLV(bytes.NewReader(buf.Bytes()))
	if err != nil || v != false {
		t.Fatalf("read err=%v v=%v", err, v)
	}
}

func TestUnsignedTLVs(t *testing.T) {
	// u8
	var buf bytes.Buffer
	if err := WriteU8TLV(&buf, 0x7F); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x02, 0x7F}) {
		t.Fatalf("u8 got %v", buf.Bytes())
	}
	if v, err := ReadU8TLV(bytes.NewReader(buf.Bytes())); err != nil || v != 0x7F {
		t.Fatalf("u8 read err=%v v=%d", err, v)
	}

	// u16
	buf.Reset()
	if err := WriteU16TLV(&buf, 0x1234); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x03, 0x34, 0x12}) {
		t.Fatalf("u16 got %v", buf.Bytes())
	}
	if v, err := ReadU16TLV(bytes.NewReader(buf.Bytes())); err != nil || v != 0x1234 {
		t.Fatalf("u16 read err=%v v=%d", err, v)
	}

	// u32 (sanity beyond tlv_test)
	buf.Reset()
	if err := WriteU32TLV(&buf, 0x89ABCDEF); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x04, 0xEF, 0xCD, 0xAB, 0x89}) {
		t.Fatalf("u32 got %v", buf.Bytes())
	}
	if v, err := ReadU32TLV(bytes.NewReader(buf.Bytes())); err != nil || v != 0x89ABCDEF {
		t.Fatalf("u32 read err=%v v=%08x", err, v)
	}

	// u64
	buf.Reset()
	if err := WriteU64TLV(&buf, 0x0102030405060708); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x05, 0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01}) {
		t.Fatalf("u64 got %v", buf.Bytes())
	}
	if v, err := ReadU64TLV(bytes.NewReader(buf.Bytes())); err != nil || v != 0x0102030405060708 {
		t.Fatalf("u64 read err=%v v=%x", err, v)
	}

	// u128
	buf.Reset()
	var u128 [16]byte
	for i := range u128 {
		u128[i] = byte(i)
	}
	if err := WriteU128TLV(&buf, u128); err != nil {
		t.Fatal(err)
	}
	want := append([]byte{0x06}, []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}...)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("u128 got %v", buf.Bytes())
	}
	if v, err := ReadU128TLV(bytes.NewReader(buf.Bytes())); err != nil || v != u128 {
		t.Fatalf("u128 read err=%v v=%v", err, v)
	}
}

func TestSignedTLVs(t *testing.T) {
	var buf bytes.Buffer
	// i8
	if err := WriteI8TLV(&buf, -1); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x07, 0xFF}) {
		t.Fatalf("i8 got %v", buf.Bytes())
	}
	if v, err := ReadI8TLV(bytes.NewReader(buf.Bytes())); err != nil || v != -1 {
		t.Fatalf("i8 read err=%v v=%d", err, v)
	}

	// i16
	buf.Reset()
	if err := WriteI16TLV(&buf, -2); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x08, 0xFE, 0xFF}) {
		t.Fatalf("i16 got %v", buf.Bytes())
	}
	if v, err := ReadI16TLV(bytes.NewReader(buf.Bytes())); err != nil || v != -2 {
		t.Fatalf("i16 read err=%v v=%d", err, v)
	}

	// i32
	buf.Reset()
	if err := WriteI32TLV(&buf, -3); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x09, 0xFD, 0xFF, 0xFF, 0xFF}) {
		t.Fatalf("i32 got %v", buf.Bytes())
	}
	if v, err := ReadI32TLV(bytes.NewReader(buf.Bytes())); err != nil || v != -3 {
		t.Fatalf("i32 read err=%v v=%d", err, v)
	}

	// i64
	buf.Reset()
	if err := WriteI64TLV(&buf, -4); err != nil {
		t.Fatal(err)
	}
	var want = []byte{0x0A}
	// two's complement of -4 in LE
	tmp := make([]byte, 8)
	binary.LittleEndian.PutUint64(tmp, uint64(^uint64(3)))
	want = append(want, tmp...)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("i64 got %v want %v", buf.Bytes(), want)
	}
	if v, err := ReadI64TLV(bytes.NewReader(buf.Bytes())); err != nil || v != -4 {
		t.Fatalf("i64 read err=%v v=%d", err, v)
	}

	// i128
	buf.Reset()
	var i128 [16]byte
	for i := range i128 {
		i128[i] = 0xAA
	}
	if err := WriteI128TLV(&buf, i128); err != nil {
		t.Fatal(err)
	}
	want = append([]byte{0x0B}, make([]byte, 16)...)
	for i := 0; i < 16; i++ {
		want[1+i] = 0xAA
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("i128 got %v", buf.Bytes())
	}
	if v, err := ReadI128TLV(bytes.NewReader(buf.Bytes())); err != nil || v != i128 {
		t.Fatalf("i128 read err=%v v=%v", err, v)
	}
}

func TestFloatAndTimestampTLVs(t *testing.T) {
	var buf bytes.Buffer
	// f32 = 1.5 => bits 0x3FC00000, LE: 00 00 C0 3F
	if err := WriteF32TLV(&buf, 1.5); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x0C, 0x00, 0x00, 0xC0, 0x3F}) {
		t.Fatalf("f32 got %v", buf.Bytes())
	}
	if v, err := ReadF32TLV(bytes.NewReader(buf.Bytes())); err != nil || v != 1.5 {
		t.Fatalf("f32 read err=%v v=%v", err, v)
	}

	// f64 = 2.5 => 00 00 00 00 00 00 04 40
	buf.Reset()
	if err := WriteF64TLV(&buf, 2.5); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), []byte{0x0D, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x40}) {
		t.Fatalf("f64 got %v", buf.Bytes())
	}
	if v, err := ReadF64TLV(bytes.NewReader(buf.Bytes())); err != nil || v != 2.5 {
		t.Fatalf("f64 read err=%v v=%v", err, v)
	}

	// timestamp
	buf.Reset()
	ts := uint64(1672531200) // 2023-01-01 00:00:00 UTC
	if err := WriteTimestampTLV(&buf, ts); err != nil {
		t.Fatal(err)
	}
	// Build expected bytes programmatically (LE u64 per SPEC.md)
	var tsb [8]byte
	binary.LittleEndian.PutUint64(tsb[:], ts)
	want := append([]byte{0x13}, tsb[:]...)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("ts got %v want %v", buf.Bytes(), want)
	}
	if v, err := ReadTimestampTLV(bytes.NewReader(buf.Bytes())); err != nil || v != ts {
		t.Fatalf("ts read err=%v v=%v", err, v)
	}
}
