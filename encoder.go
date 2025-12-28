package relish

import (
	"io"
	"reflect"
	"sort"

	intr "github.com/dadrian/relish/internal"
)

// Encoder writes Relish-encoded values to an io.Writer.
type Encoder struct {
	w io.Writer
}

// NewEncoder creates a new streaming encoder.
func NewEncoder(w io.Writer) *Encoder { return &Encoder{w: w} }

// Encode writes the TLV for v.
func (e *Encoder) Encode(v any) error { return e.encodeValue(reflect.ValueOf(v)) }

// Convenience primitive writers for fixed-size types.
func (e *Encoder) WriteNull() error         { return intr.WriteNullTLV(e.w) }
func (e *Encoder) WriteBool(v bool) error   { return intr.WriteBoolTLV(e.w, v) }
func (e *Encoder) WriteU8(v uint8) error    { return intr.WriteU8TLV(e.w, v) }
func (e *Encoder) WriteU16(v uint16) error  { return intr.WriteU16TLV(e.w, v) }
func (e *Encoder) WriteU32(v uint32) error  { return intr.WriteU32TLV(e.w, v) }
func (e *Encoder) WriteU64(v uint64) error  { return intr.WriteU64TLV(e.w, v) }
func (e *Encoder) WriteU128(v U128) error   { return intr.WriteU128TLV(e.w, [16]byte(v)) }
func (e *Encoder) WriteI8(v int8) error     { return intr.WriteI8TLV(e.w, v) }
func (e *Encoder) WriteI16(v int16) error   { return intr.WriteI16TLV(e.w, v) }
func (e *Encoder) WriteI32(v int32) error   { return intr.WriteI32TLV(e.w, v) }
func (e *Encoder) WriteI64(v int64) error   { return intr.WriteI64TLV(e.w, v) }
func (e *Encoder) WriteI128(v I128) error   { return intr.WriteI128TLV(e.w, [16]byte(v)) }
func (e *Encoder) WriteF32(v float32) error { return intr.WriteF32TLV(e.w, v) }
func (e *Encoder) WriteF64(v float64) error { return intr.WriteF64TLV(e.w, v) }

// Varsize stubs remain unimplemented for now.
func (e *Encoder) WriteString(s string) error { return intr.WriteStringTLV(e.w, s) }
func (e *Encoder) WriteArray(elems any) error { return ErrNotImplemented }
func (e *Encoder) WriteMap(m any) error       { return ErrNotImplemented }

// encodeValue writes the TLV for v.
func (e *Encoder) encodeValue(rv reflect.Value) error {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			// nil pointer encodes as zero value of element
			rv = reflect.Zero(rv.Type().Elem())
			break
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Bool:
		return intr.WriteBoolTLV(e.w, rv.Bool())
	case reflect.Uint8:
		return intr.WriteU8TLV(e.w, uint8(rv.Uint()))
	case reflect.Uint16:
		return intr.WriteU16TLV(e.w, uint16(rv.Uint()))
	case reflect.Uint32:
		return intr.WriteU32TLV(e.w, uint32(rv.Uint()))
	case reflect.Uint64:
		return intr.WriteU64TLV(e.w, uint64(rv.Uint()))
	case reflect.Int8:
		return intr.WriteI8TLV(e.w, int8(rv.Int()))
	case reflect.Int16:
		return intr.WriteI16TLV(e.w, int16(rv.Int()))
	case reflect.Int32:
		return intr.WriteI32TLV(e.w, int32(rv.Int()))
	case reflect.Int64:
		return intr.WriteI64TLV(e.w, int64(rv.Int()))
	case reflect.Float32:
		return intr.WriteF32TLV(e.w, float32(rv.Float()))
	case reflect.Float64:
		return intr.WriteF64TLV(e.w, float64(rv.Float()))
	case reflect.String:
		return intr.WriteStringTLV(e.w, rv.String())
	case reflect.Struct:
		return e.encodeStruct(rv)
	default:
		return ErrNotImplemented
	}
}

func (e *Encoder) encodeStruct(rv reflect.Value) error {
	rt := rv.Type()
	type fieldInfo struct {
		id        int
		optional  bool
		omitempty bool
		value     reflect.Value
	}
	var fields []fieldInfo
	var optCount, presentOpt int
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		id, optional, omitempty, ok := intr.ParseRelishTag(f)
		if !ok {
			continue
		}
		fv := rv.Field(i)
		if optional {
			optCount++
			if fv.Kind() == reflect.Pointer && !fv.IsNil() {
				presentOpt++
			}
		}
		fields = append(fields, fieldInfo{id: id, optional: optional, omitempty: omitempty, value: fv})
	}
	// Enum-like: all optional and exactly one present
	if len(fields) > 0 && optCount == len(fields) && presentOpt == 1 {
		for _, fi := range fields {
			fv := fi.value
			if fv.Kind() == reflect.Pointer && !fv.IsNil() {
				return intr.WriteEnumTLV(e.w, byte(fi.id), func(w io.Writer) error {
					return NewEncoder(w).encodeValue(fv)
				})
			}
		}
	}
	// Struct encoding: write fields in increasing ID order
	sort.Slice(fields, func(i, j int) bool { return fields[i].id < fields[j].id })
	return intr.WriteStructTLV(e.w, func(w io.Writer) error {
		enc := NewEncoder(w)
		for _, fi := range fields {
			fv := fi.value
			if fi.optional && fv.Kind() == reflect.Pointer && fv.IsNil() {
				continue
			}
			if fi.omitempty && isZeroValue(fv) {
				continue
			}
			if err := intr.WriteType(w, byte(fi.id)); err != nil {
				return err
			}
			if err := enc.encodeValue(fv); err != nil {
				return err
			}
		}
		return nil
	})
}

func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		return v.IsNil()
	default:
		z := reflect.Zero(v.Type())
		return reflect.DeepEqual(v.Interface(), z.Interface())
	}
}
