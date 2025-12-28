package internal

import (
	"encoding/binary"
	"errors"
	"io"
	"unicode/utf8"
)

var errInvalidTypeID = errors.New("invalid type id (top bit set)")

// IsVarSize reports whether a type ID is varsize per SPEC.md.
func IsVarSize(t byte) bool {
	switch t {
	case 0x0E, 0x0F, 0x10, 0x11, 0x12:
		return true
	default:
		return false
	}
}

// FixedSize returns the number of content bytes for a fixed-size type.
// It returns (0, true) for Null and (0, false) for varsize/unknown types.
func FixedSize(t byte) (int, bool) {
	switch t {
	case 0x00: // Null
		return 0, true
	case 0x01: // Bool
		return 1, true
	case 0x02: // u8
		return 1, true
	case 0x03: // u16
		return 2, true
	case 0x04: // u32
		return 4, true
	case 0x05: // u64
		return 8, true
	case 0x06: // u128
		return 16, true
	case 0x07: // i8
		return 1, true
	case 0x08: // i16
		return 2, true
	case 0x09: // i32
		return 4, true
	case 0x0A: // i64
		return 8, true
	case 0x0B: // i128
		return 16, true
	case 0x0C: // f32
		return 4, true
	case 0x0D: // f64
		return 8, true
	case 0x13: // Timestamp (u64 seconds)
		return 8, true
	default:
		return 0, false
	}
}

// WriteType writes a single validated type ID byte.
func WriteType(w io.Writer, t byte) error {
	if t&0x80 != 0 {
		return errInvalidTypeID
	}
	_, err := w.Write([]byte{t})
	return err
}

// ReadType reads and validates a single type ID byte.
func ReadType(r io.Reader) (byte, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	if b[0]&0x80 != 0 {
		return 0, errInvalidTypeID
	}
	return b[0], nil
}

// WriteLen writes a tagged-varint length using a small stack buffer.
func WriteLen(w io.Writer, n int) (int, error) {
	sz := SizeOfLen(n)
	if sz < 0 {
		return 0, errors.New("length out of range")
	}
	var buf [4]byte
	nn := EncodeLen(buf[:], n)
	_, err := w.Write(buf[:nn])
	return nn, err
}

// ReadLen reads a tagged-varint length from r.
func ReadLen(r io.Reader) (int, int, error) {
	var b [4]byte
	// Peek first byte to decide short/long form
	if _, err := io.ReadFull(r, b[:1]); err != nil {
		return -1, 0, err
	}
	if b[0]&0x01 == 0 {
		n, _ := DecodeLen(b[:1])
		return n, 1, nil
	}
	if _, err := io.ReadFull(r, b[1:4]); err != nil {
		return -1, 0, err
	}
	n, _ := DecodeLen(b[:4])
	if n < 0 {
		return -1, 0, errors.New("invalid length")
	}
	return n, 4, nil
}

// WriteU32TLV writes a u32 TLV: [0x04][LE u32].
func WriteU32TLV(w io.Writer, v uint32) error {
	if err := WriteType(w, 0x04); err != nil {
		return err
	}
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

// ReadU32TLV reads a u32 TLV and returns the value.
func ReadU32TLV(r io.Reader) (uint32, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, err
	}
	if t != 0x04 {
		return 0, errors.New("unexpected type id for u32")
	}
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

// WriteStringTLV writes a string TLV: [0x0E][len][UTF-8 bytes].
// Validates that the input is valid UTF-8.
func WriteStringTLV(w io.Writer, s string) error {
	if !utf8.ValidString(s) {
		return errors.New("invalid utf-8")
	}
	if err := WriteType(w, 0x0E); err != nil {
		return err
	}
	if _, err := WriteLen(w, len(s)); err != nil {
		return err
	}
	if len(s) == 0 {
		return nil
	}
	_, err := w.Write([]byte(s))
	return err
}

// ReadStringTLV reads a string TLV and returns the string.
// Validates the input bytes are valid UTF-8.
func ReadStringTLV(r io.Reader) (string, error) {
	t, err := ReadType(r)
	if err != nil {
		return "", err
	}
	if t != 0x0E {
		return "", errors.New("unexpected type id for string")
	}
	n, _, err := ReadLen(r)
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	buf := make([]byte, n)
	if err := ReadFull(r, buf); err != nil {
		return "", err
	}
	if !utf8.Valid(buf) {
		return "", errors.New("invalid utf-8")
	}
	return string(buf), nil
}

// WriteArrayTLV writes an array TLV.
// Layout: [0x0F][len][element_type_id][elements...]
// The writeElems closure should write element content only:
// - For fixed-size element types: raw value bytes for each element
// - For varsize element types: [len][content] for each element (no type byte)
func WriteArrayTLV(w io.Writer, elemType byte, writeElems func(io.Writer) error) error {
	if elemType&0x80 != 0 {
		return errInvalidTypeID
	}
	// Buffer content to compute length
	buf := GetBuffer()
	defer PutBuffer(buf)
	// element type id
	if err := WriteType(buf, elemType); err != nil {
		return err
	}
	if err := writeElems(buf); err != nil {
		return err
	}
	if err := WriteType(w, 0x0F); err != nil {
		return err
	}
	if _, err := WriteLen(w, buf.Len()); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// ReadArrayTLV reads an array TLV and returns the element type ID and the raw element payload bytes.
// The returned payload excludes the element_type_id and contains only the concatenated element encodings.
func ReadArrayTLV(r io.Reader) (byte, []byte, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, nil, err
	}
	if t != 0x0F {
		return 0, nil, errors.New("unexpected type id for array")
	}
	n, _, err := ReadLen(r)
	if err != nil {
		return 0, nil, err
	}
	if n < 1 {
		return 0, nil, errors.New("array content too short")
	}
	buf := make([]byte, n)
	if err := ReadFull(r, buf); err != nil {
		return 0, nil, err
	}
	elemType := buf[0]
	if elemType&0x80 != 0 {
		return 0, nil, errInvalidTypeID
	}
	payload := buf[1:]
	return elemType, payload, nil
}

// WriteStructTLV writes a struct TLV.
// Layout: [0x11][len][fields...]
// The writeFields closure must write a sequence of fields as [field_id][field_value TLV].
// Field IDs must have top bit clear.
func WriteStructTLV(w io.Writer, writeFields func(io.Writer) error) error {
	buf := GetBuffer()
	defer PutBuffer(buf)
	if err := writeFields(buf); err != nil {
		return err
	}
	if err := WriteType(w, 0x11); err != nil {
		return err
	}
	if _, err := WriteLen(w, buf.Len()); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// ReadStructTLV reads a struct TLV and returns the raw field payload bytes.
func ReadStructTLV(r io.Reader) ([]byte, error) {
	t, err := ReadType(r)
	if err != nil {
		return nil, err
	}
	if t != 0x11 {
		return nil, errors.New("unexpected type id for struct")
	}
	n, _, err := ReadLen(r)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	buf := make([]byte, n)
	if err := ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// WriteEnumTLV writes an enum TLV.
// Layout: [0x12][len][variant_id][variant_value TLV]
func WriteEnumTLV(w io.Writer, variantID byte, writeVariant func(io.Writer) error) error {
	if variantID&0x80 != 0 {
		return errInvalidTypeID
	}
	buf := GetBuffer()
	defer PutBuffer(buf)
	// variant id
	if _, err := buf.Write([]byte{variantID}); err != nil {
		return err
	}
	if err := writeVariant(buf); err != nil {
		return err
	}
	if err := WriteType(w, 0x12); err != nil {
		return err
	}
	if _, err := WriteLen(w, buf.Len()); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// ReadEnumTLV reads an enum TLV and returns the variant ID and its value payload
// (starting at the variant value's type byte, i.e., a full TLV).
func ReadEnumTLV(r io.Reader) (byte, []byte, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, nil, err
	}
	if t != 0x12 {
		return 0, nil, errors.New("unexpected type id for enum")
	}
	n, _, err := ReadLen(r)
	if err != nil {
		return 0, nil, err
	}
	if n < 1 {
		return 0, nil, errors.New("enum content too short")
	}
	buf := make([]byte, n)
	if err := ReadFull(r, buf); err != nil {
		return 0, nil, err
	}
	variantID := buf[0]
	if variantID&0x80 != 0 {
		return 0, nil, errInvalidTypeID
	}
	return variantID, buf[1:], nil
}

// WriteMapTLV writes a map TLV.
// Layout: [0x10][len][key_type_id][value_type_id][pairs...]
// The writePairs closure should write key/value encodings only (without type bytes):
// - For fixed-size types: raw value bytes
// - For varsize types: [len][content]
func WriteMapTLV(w io.Writer, keyType, valueType byte, writePairs func(io.Writer) error) error {
	if keyType&0x80 != 0 || valueType&0x80 != 0 {
		return errInvalidTypeID
	}
	// Buffer the content to compute length
	buf := GetBuffer()
	defer PutBuffer(buf)
	if err := WriteType(buf, keyType); err != nil {
		return err
	}
	if err := WriteType(buf, valueType); err != nil {
		return err
	}
	if err := writePairs(buf); err != nil {
		return err
	}
	if err := WriteType(w, 0x10); err != nil {
		return err
	}
	if _, err := WriteLen(w, buf.Len()); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// ReadMapTLV reads a map TLV and returns key/value type IDs and the raw pair payload bytes.
// The payload excludes the leading key_type_id and value_type_id bytes.
func ReadMapTLV(r io.Reader) (byte, byte, []byte, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, 0, nil, err
	}
	if t != 0x10 {
		return 0, 0, nil, errors.New("unexpected type id for map")
	}
	n, _, err := ReadLen(r)
	if err != nil {
		return 0, 0, nil, err
	}
	if n < 2 {
		return 0, 0, nil, errors.New("map content too short")
	}
	buf := make([]byte, n)
	if err := ReadFull(r, buf); err != nil {
		return 0, 0, nil, err
	}
	kt := buf[0]
	vt := buf[1]
	if kt&0x80 != 0 || vt&0x80 != 0 {
		return 0, 0, nil, errInvalidTypeID
	}
	payload := buf[2:]
	return kt, vt, payload, nil
}
