package internal

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
)

// Null TLV: [0x00]
func WriteNullTLV(w io.Writer) error {
	return WriteType(w, 0x00)
}

func ReadNullTLV(r io.Reader) error {
	t, err := ReadType(r)
	if err != nil {
		return err
	}
	if t != 0x00 {
		return errors.New("unexpected type id for null")
	}
	return nil
}

// Bool TLV: [0x01][0x00|0xFF]
func WriteBoolTLV(w io.Writer, v bool) error {
	if err := WriteType(w, 0x01); err != nil {
		return err
	}
	b := byte(0x00)
	if v {
		b = 0xFF
	}
	_, err := w.Write([]byte{b})
	return err
}

func ReadBoolTLV(r io.Reader) (bool, error) {
	t, err := ReadType(r)
	if err != nil {
		return false, err
	}
	if t != 0x01 {
		return false, errors.New("unexpected type id for bool")
	}
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return false, err
	}
	switch b[0] {
	case 0x00:
		return false, nil
	case 0xFF:
		return true, nil
	default:
		return false, errors.New("invalid bool value")
	}
}

// Unsigned integers
func WriteU8TLV(w io.Writer, v uint8) error {
	if err := WriteType(w, 0x02); err != nil {
		return err
	}
	_, err := w.Write([]byte{byte(v)})
	return err
}
func ReadU8TLV(r io.Reader) (uint8, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, err
	}
	if t != 0x02 {
		return 0, errors.New("unexpected type id for u8")
	}
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return b[0], nil
}

func WriteU16TLV(w io.Writer, v uint16) error {
	if err := WriteType(w, 0x03); err != nil {
		return err
	}
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	_, err := w.Write(b[:])
	return err
}
func ReadU16TLV(r io.Reader) (uint16, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, err
	}
	if t != 0x03 {
		return 0, errors.New("unexpected type id for u16")
	}
	var b [2]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(b[:]), nil
}

// WriteU32TLV/ReadU32TLV are in tlv.go

func WriteU64TLV(w io.Writer, v uint64) error {
	if err := WriteType(w, 0x05); err != nil {
		return err
	}
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	_, err := w.Write(b[:])
	return err
}
func ReadU64TLV(r io.Reader) (uint64, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, err
	}
	if t != 0x05 {
		return 0, errors.New("unexpected type id for u64")
	}
	var b [8]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

func WriteU128TLV(w io.Writer, v [16]byte) error {
	if err := WriteType(w, 0x06); err != nil {
		return err
	}
	_, err := w.Write(v[:])
	return err
}
func ReadU128TLV(r io.Reader) ([16]byte, error) {
	var out [16]byte
	t, err := ReadType(r)
	if err != nil {
		return out, err
	}
	if t != 0x06 {
		return out, errors.New("unexpected type id for u128")
	}
	if _, err := io.ReadFull(r, out[:]); err != nil {
		return out, err
	}
	return out, nil
}

// Signed integers
func WriteI8TLV(w io.Writer, v int8) error {
	if err := WriteType(w, 0x07); err != nil {
		return err
	}
	_, err := w.Write([]byte{byte(v)})
	return err
}
func ReadI8TLV(r io.Reader) (int8, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, err
	}
	if t != 0x07 {
		return 0, errors.New("unexpected type id for i8")
	}
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return int8(b[0]), nil
}

func WriteI16TLV(w io.Writer, v int16) error {
	if err := WriteType(w, 0x08); err != nil {
		return err
	}
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], uint16(v))
	_, err := w.Write(b[:])
	return err
}
func ReadI16TLV(r io.Reader) (int16, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, err
	}
	if t != 0x08 {
		return 0, errors.New("unexpected type id for i16")
	}
	var b [2]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(b[:])), nil
}

func WriteI32TLV(w io.Writer, v int32) error {
	if err := WriteType(w, 0x09); err != nil {
		return err
	}
	var b [4]byte
	// Go stdlib lacks PutInt32; cast to uint32 for two's complement bytes.
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	_, err := w.Write(b[:])
	return err
}
func ReadI32TLV(r io.Reader) (int32, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, err
	}
	if t != 0x09 {
		return 0, errors.New("unexpected type id for i32")
	}
	var b [4]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(b[:])), nil
}

func WriteI64TLV(w io.Writer, v int64) error {
	if err := WriteType(w, 0x0A); err != nil {
		return err
	}
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(v))
	_, err := w.Write(b[:])
	return err
}

func ReadI64TLV(r io.Reader) (int64, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, err
	}
	if t != 0x0A {
		return 0, errors.New("unexpected type id for i64")
	}
	var b [8]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(b[:])), nil
}

func WriteI128TLV(w io.Writer, v [16]byte) error {
	if err := WriteType(w, 0x0B); err != nil {
		return err
	}
	_, err := w.Write(v[:])
	return err
}
func ReadI128TLV(r io.Reader) ([16]byte, error) {
	var out [16]byte
	t, err := ReadType(r)
	if err != nil {
		return out, err
	}
	if t != 0x0B {
		return out, errors.New("unexpected type id for i128")
	}
	if _, err := io.ReadFull(r, out[:]); err != nil {
		return out, err
	}
	return out, nil
}

// Floats
func WriteF32TLV(w io.Writer, v float32) error {
	if err := WriteType(w, 0x0C); err != nil {
		return err
	}
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
	_, err := w.Write(b[:])
	return err
}
func ReadF32TLV(r io.Reader) (float32, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, err
	}
	if t != 0x0C {
		return 0, errors.New("unexpected type id for f32")
	}
	var b [4]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(b[:])), nil
}

func WriteF64TLV(w io.Writer, v float64) error {
	if err := WriteType(w, 0x0D); err != nil {
		return err
	}
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
	_, err := w.Write(b[:])
	return err
}
func ReadF64TLV(r io.Reader) (float64, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, err
	}
	if t != 0x0D {
		return 0, errors.New("unexpected type id for f64")
	}
	var b [8]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(b[:])), nil
}

// Timestamp (u64 seconds)
func WriteTimestampTLV(w io.Writer, v uint64) error {
	if err := WriteType(w, 0x13); err != nil {
		return err
	}
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	_, err := w.Write(b[:])
	return err
}
func ReadTimestampTLV(r io.Reader) (uint64, error) {
	t, err := ReadType(r)
	if err != nil {
		return 0, err
	}
	if t != 0x13 {
		return 0, errors.New("unexpected type id for timestamp")
	}
	var b [8]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}
