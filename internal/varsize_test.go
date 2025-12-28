package internal

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func TestStringTLV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteStringTLV(&buf, "hello"); err != nil {
		t.Fatalf("write string: %v", err)
	}
	want := append([]byte{0x0E, 0x0A}, []byte("hello")...)
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("encoded mismatch: got %v want %v", got, want)
	}
	s, err := ReadStringTLV(bytes.NewReader(buf.Bytes()))
	if err != nil || s != "hello" {
		t.Fatalf("read err=%v s=%q", err, s)
	}
}

func TestArrayTLV_FixedElements(t *testing.T) {
	var buf bytes.Buffer
	// Array of two u32: 1, 2
	err := WriteArrayTLV(&buf, 0x04, func(w io.Writer) error {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], 1)
		if _, err := w.Write(b[:]); err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(b[:], 2)
		_, err := w.Write(b[:])
		return err
	})
	if err != nil {
		t.Fatalf("write array: %v", err)
	}
	// Expect: [0x0F][len=0x12][0x04][1][2]
	want := []byte{0x0F, 0x12, 0x04, 0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00}
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("encoded mismatch: got %v want %v", got, want)
	}
	// Now read back
	elemType, payload, err := ReadArrayTLV(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read array: %v", err)
	}
	if elemType != 0x04 {
		t.Fatalf("elemType=%02x", elemType)
	}
	if len(payload) != 8 {
		t.Fatalf("payload len=%d", len(payload))
	}
	v0 := binary.LittleEndian.Uint32(payload[0:4])
	v1 := binary.LittleEndian.Uint32(payload[4:8])
	if v0 != 1 || v1 != 2 {
		t.Fatalf("values got (%d,%d)", v0, v1)
	}
}

func TestArrayTLV_VarElementsString(t *testing.T) {
	var buf bytes.Buffer
	// Array of two strings: "a", "bc"
	err := WriteArrayTLV(&buf, 0x0E, func(w io.Writer) error {
		if _, err := WriteLen(w, 1); err != nil {
			return err
		}
		if _, err := w.Write([]byte("a")); err != nil {
			return err
		}
		if _, err := WriteLen(w, 2); err != nil {
			return err
		}
		_, err := w.Write([]byte("bc"))
		return err
	})
	if err != nil {
		t.Fatalf("write array: %v", err)
	}
	// content: [0x0E][0x02]['a'][0x04]['b''c'] -> total content=6 -> len byte=0x0C
	want := []byte{0x0F, 0x0C /* len=6 */, 0x0E, 0x02, 'a', 0x04, 'b', 'c'}
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("encoded mismatch: got %v want %v", got, want)
	}
	et, payload, err := ReadArrayTLV(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read array: %v", err)
	}
	if et != 0x0E {
		t.Fatalf("elemType=%02x", et)
	}
	if !bytes.Equal(payload, []byte{0x02, 'a', 0x04, 'b', 'c'}) {
		t.Fatalf("payload got %v", payload)
	}
}

func TestMapTLV_FixedElements(t *testing.T) {
	var buf bytes.Buffer
	// Map<u32,u32> with two pairs: (1->2), (3->4)
	err := WriteMapTLV(&buf, 0x04, 0x04, func(w io.Writer) error {
		var b [4]byte
		// k=1, v=2
		binary.LittleEndian.PutUint32(b[:], 1)
		if _, err := w.Write(b[:]); err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(b[:], 2)
		if _, err := w.Write(b[:]); err != nil {
			return err
		}
		// k=3, v=4
		binary.LittleEndian.PutUint32(b[:], 3)
		if _, err := w.Write(b[:]); err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(b[:], 4)
		_, err := w.Write(b[:])
		return err
	})
	if err != nil {
		t.Fatalf("write map: %v", err)
	}
	// content: [0x04][0x04][1][2][3][4] -> 2 + 16 = 18 bytes -> short len: 18<<1 = 36 = 0x24
	// final: [0x10][0x24][0x04][0x04] + four u32
	want := []byte{0x10, 0x24, 0x04, 0x04,
		0x01, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x00, 0x00,
		0x03, 0x00, 0x00, 0x00,
		0x04, 0x00, 0x00, 0x00,
	}
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("encoded mismatch: got %v want %v", got, want)
	}
	kt, vt, payload, err := ReadMapTLV(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read map: %v", err)
	}
	if kt != 0x04 || vt != 0x04 {
		t.Fatalf("types got (%02x,%02x)", kt, vt)
	}
	if len(payload) != 16 {
		t.Fatalf("payload len=%d", len(payload))
	}
	k0 := binary.LittleEndian.Uint32(payload[0:4])
	v0 := binary.LittleEndian.Uint32(payload[4:8])
	k1 := binary.LittleEndian.Uint32(payload[8:12])
	v1 := binary.LittleEndian.Uint32(payload[12:16])
	if k0 != 1 || v0 != 2 || k1 != 3 || v1 != 4 {
		t.Fatalf("pairs got (%d->%d, %d->%d)", k0, v0, k1, v1)
	}
}

func TestMapTLV_VarElements(t *testing.T) {
	var buf bytes.Buffer
	// Map<string,string>: {"a":"x", "bb":"yz"}
	err := WriteMapTLV(&buf, 0x0E, 0x0E, func(w io.Writer) error {
		// k="a", v="x"
		if _, err := WriteLen(w, 1); err != nil {
			return err
		}
		if _, err := w.Write([]byte("a")); err != nil {
			return err
		}
		if _, err := WriteLen(w, 1); err != nil {
			return err
		}
		if _, err := w.Write([]byte("x")); err != nil {
			return err
		}
		// k="bb", v="yz"
		if _, err := WriteLen(w, 2); err != nil {
			return err
		}
		if _, err := w.Write([]byte("bb")); err != nil {
			return err
		}
		if _, err := WriteLen(w, 2); err != nil {
			return err
		}
		_, err := w.Write([]byte("yz"))
		return err
	})
	if err != nil {
		t.Fatalf("write map: %v", err)
	}
	// content: [0x0E][0x0E] + [0x02 'a'][0x02 'x'][0x04 'bb'][0x04 'yz']
	// sizes: 2 (types) + 2 + 2 + 3 + 3 = 12 -> short len: 12<<1=24=0x18
	want := []byte{0x10, 0x18, 0x0E, 0x0E, 0x02, 'a', 0x02, 'x', 0x04, 'b', 'b', 0x04, 'y', 'z'}
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("encoded mismatch: got %v want %v", got, want)
	}
	kt, vt, payload, err := ReadMapTLV(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read map: %v", err)
	}
	if kt != 0x0E || vt != 0x0E {
		t.Fatalf("types got (%02x,%02x)", kt, vt)
	}
	if !bytes.Equal(payload, []byte{0x02, 'a', 0x02, 'x', 0x04, 'b', 'b', 0x04, 'y', 'z'}) {
		t.Fatalf("payload got %v", payload)
	}
}

func TestStructTLV_Simple(t *testing.T) {
	var buf bytes.Buffer
	// Struct with one field: id=0, value=u32(42)
	err := WriteStructTLV(&buf, func(w io.Writer) error {
		// field id 0
		if _, err := w.Write([]byte{0x00}); err != nil {
			return err
		}
		// u32 TLV
		return WriteU32TLV(w, 42)
	})
	if err != nil {
		t.Fatalf("write struct: %v", err)
	}
	// Expect: [0x11][0x0C][0x00][0x04][0x2A 00 00 00]
	want := []byte{0x11, 0x0C, 0x00, 0x04, 0x2A, 0x00, 0x00, 0x00}
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("encoded mismatch: got %v want %v", got, want)
	}
	payload, err := ReadStructTLV(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read struct: %v", err)
	}
	if !bytes.Equal(payload, []byte{0x00, 0x04, 0x2A, 0x00, 0x00, 0x00}) {
		t.Fatalf("payload got %v", payload)
	}
}

func TestEnumTLV_Simple(t *testing.T) {
	var buf bytes.Buffer
	// Enum variant 0 with value u32(10)
	err := WriteEnumTLV(&buf, 0x00, func(w io.Writer) error {
		return WriteU32TLV(w, 10)
	})
	if err != nil {
		t.Fatalf("write enum: %v", err)
	}
	// Expect: [0x12][0x0C][0x00][0x04][0x0A 00 00 00]
	want := []byte{0x12, 0x0C, 0x00, 0x04, 0x0A, 0x00, 0x00, 0x00}
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("encoded mismatch: got %v want %v", got, want)
	}
	vid, payload, err := ReadEnumTLV(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read enum: %v", err)
	}
	if vid != 0x00 {
		t.Fatalf("variant id=%02x", vid)
	}
	if !bytes.Equal(payload, []byte{0x04, 0x0A, 0x00, 0x00, 0x00}) {
		t.Fatalf("payload got %v", payload)
	}
}
