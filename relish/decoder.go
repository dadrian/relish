package relish

import (
	"bytes"
	"io"
	"reflect"

	intr "github.com/dadrian/relish/relish/internal"
)

// Decoder reads Relish-encoded values from an io.Reader.
type Decoder struct {
	r      io.Reader
	offset int64
}

// NewDecoder creates a new streaming decoder.
func NewDecoder(r io.Reader) *Decoder { return &Decoder{r: r} }

// Decode reads a TLV into v. This is a stub for now.
func (d *Decoder) Decode(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &Error{Kind: ErrTypeMismatch, Detail: "Decode target must be non-nil pointer"}
	}
	rv = rv.Elem()
	switch rv.Kind() {
	case reflect.Struct:
		// Peek type for struct vs enum
		t, err := intr.ReadType(d.r)
		if err != nil {
			return err
		}
		switch t {
		case 0x11:
			return d.decodeStructInto(rv)
		case 0x12:
			return d.decodeEnumInto(rv)
		default:
			return &Error{Kind: ErrTypeMismatch, Detail: "expected struct/enum TLV"}
		}
	case reflect.String:
		s, err := intr.ReadStringTLV(d.r)
		if err != nil {
			return err
		}
		rv.SetString(s)
		return nil
	case reflect.Bool:
		b, err := intr.ReadBoolTLV(d.r)
		if err != nil {
			return err
		}
		rv.SetBool(b)
		return nil
	case reflect.Uint32:
		u, err := intr.ReadU32TLV(d.r)
		if err != nil {
			return err
		}
		rv.SetUint(uint64(u))
		return nil
	default:
		return ErrNotImplemented
	}
}

// SkipValue efficiently skips a single TLV value. Stub for now.
func (d *Decoder) SkipValue() error { return ErrNotImplemented }

func (d *Decoder) decodeStructInto(dst reflect.Value) error {
	// We already consumed type byte in Decode; next is length
	n, _, err := intr.ReadLen(d.r)
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	buf := make([]byte, n)
	if err := intr.ReadFull(d.r, buf); err != nil {
		return err
	}
	br := bytes.NewReader(buf)
	// Build field id -> index map
	rt := dst.Type()
	idToIndex := make(map[int]int)
	for i := 0; i < rt.NumField(); i++ {
		if id, _, _, ok := intr.ParseRelishTag(rt.Field(i)); ok {
			idToIndex[id] = i
		}
	}
	var prev = -1
	for br.Len() > 0 {
		// Field ID
		b, err := br.ReadByte()
		if err != nil {
			return err
		}
		if b&0x80 != 0 {
			return &Error{Kind: ErrInvalidFieldID, Detail: "top bit set"}
		}
		id := int(b)
		if id <= prev {
			return &Error{Kind: ErrFieldOrder, Detail: "field ids not strictly increasing"}
		}
		prev = id
		// Read full TLV for this field
		tlv, err := readTLVBytes(br)
		if err != nil {
			return err
		}
		idx, ok := idToIndex[id]
		if !ok {
			// unknown field: ignore
			continue
		}
		f := dst.Field(idx)
		// Handle pointer optional fields
		if f.Kind() == reflect.Pointer {
			if f.IsNil() {
				f.Set(reflect.New(f.Type().Elem()))
			}
			f = f.Elem()
		}
		// Decode into field from TLV bytes
		if err := NewDecoder(bytes.NewReader(tlv)).Decode(f.Addr().Interface()); err != nil {
			return err
		}
	}
	return nil
}

func (d *Decoder) decodeEnumInto(dst reflect.Value) error {
	// Type already consumed by caller
	n, _, err := intr.ReadLen(d.r)
	if err != nil {
		return err
	}
	if n < 1 {
		return &Error{Kind: ErrTypeMismatch, Detail: "enum content too short"}
	}
	buf := make([]byte, n)
	if err := intr.ReadFull(d.r, buf); err != nil {
		return err
	}
	vid := buf[0]
	payload := buf[1:]
	rt := dst.Type()
	var idx = -1
	for i := 0; i < rt.NumField(); i++ {
		if id, _, _, ok := intr.ParseRelishTag(rt.Field(i)); ok && byte(id) == vid {
			idx = i
			break
		}
	}
	if idx < 0 {
		return &Error{Kind: ErrTypeMismatch, Detail: "unknown enum variant"}
	}
	// Decode variant value; must consume entire payload
	br := bytes.NewReader(payload)
	f := dst.Field(idx)
	if f.Kind() != reflect.Pointer {
		// enum fields must be pointers in these tests
		return &Error{Kind: ErrTypeMismatch, Detail: "enum field must be pointer"}
	}
	if f.IsNil() {
		f.Set(reflect.New(f.Type().Elem()))
	}
	if err := NewDecoder(br).Decode(f.Interface()); err != nil {
		return err
	}
	if br.Len() != 0 {
		return &Error{Kind: ErrEnumLengthMismatch, Detail: "variant did not consume full length"}
	}
	return nil
}

// readTLVBytes reads a complete TLV (type + [len] + content) and returns its bytes.
func readTLVBytes(r io.Reader) ([]byte, error) {
	t, err := intr.ReadType(r)
	if err != nil {
		return nil, err
	}
	if n, ok := intr.FixedSize(t); ok {
		if n == 0 {
			return []byte{t}, nil
		}
		out := make([]byte, 1+n)
		out[0] = t
		if err := intr.ReadFull(r, out[1:]); err != nil {
			return nil, err
		}
		return out, nil
	}
	// varsize
	n, used, err := intr.ReadLen(r)
	if err != nil {
		return nil, err
	}
	hdr := make([]byte, 1+used)
	hdr[0] = t
	// Re-encode the length
	var tmp [4]byte
	_ = intr.EncodeLen(tmp[:], n)
	copy(hdr[1:], tmp[:used])
	out := make([]byte, len(hdr)+n)
	copy(out, hdr)
	if n > 0 {
		if err := intr.ReadFull(r, out[len(hdr):]); err != nil {
			return nil, err
		}
	}
	return out, nil
}
