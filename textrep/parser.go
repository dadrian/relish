package textrep

import (
    "bytes"
    "errors"
    "fmt"
    "io"
    "sort"
    "strconv"
    "strings"
    "time"

    intr "github.com/dadrian/relish/internal"
)

// Public API

// Encode reads an RTR document from r and writes a Relish Struct TLV to w.
func Encode(r io.Reader, w io.Writer) error {
    src, err := io.ReadAll(r)
    if err != nil { return err }
    out, err := EncodeBytes(src)
    if err != nil { return err }
    _, err = w.Write(out)
    return err
}

// EncodeBytes parses an RTR document and returns its Relish Struct TLV bytes.
func EncodeBytes(src []byte) ([]byte, error) {
    p := &parser{lx: newLexer(src)}
    // parse preamble aliases
    aliases := map[string]alias{}
    p.lx.next()
    for p.lx.cur.kind == tokLet {
        a, err := p.parseAlias()
        if err != nil { return nil, err }
        if a.id < 0 || a.id >= 0x80 { return nil, fmt.Errorf("alias id out of range: %d", a.id) }
        if _, exists := aliases[a.name]; exists { return nil, fmt.Errorf("duplicate alias: %s", a.name) }
        aliases[a.name] = a
    }
    // expect top-level struct
    st, err := p.parseStructLiteral(aliases)
    if err != nil { return nil, err }
    // encode struct
    var buf bytes.Buffer
    err = intr.WriteStructTLV(&buf, func(w io.Writer) error {
        // sort by field id
        sort.Slice(st.fields, func(i, j int) bool { return st.fields[i].id < st.fields[j].id })
        seen := map[int]struct{}{}
        for _, f := range st.fields {
            if f.omit { continue }
            if _, ok := seen[f.id]; ok { return fmt.Errorf("duplicate field id: %d", f.id) }
            seen[f.id] = struct{}{}
            if f.id < 0 || f.id >= 0x80 { return fmt.Errorf("field id out of range: %d", f.id) }
            if err := intr.WriteType(w, byte(f.id)); err != nil { return err }
            tlv, err := encodeValueTLV(f.val)
            if err != nil { return err }
            _, err = w.Write(tlv)
            if err != nil { return err }
        }
        return nil
    })
    if err != nil { return nil, err }
    return buf.Bytes(), nil
}

// Internal AST and parser

type alias struct {
    name string
    id   int
    typ  *rType // optional type hint
}

// rType models Relish types (subset sufficient for encoding decisions)
type rTypeKind int

const (
    tNull rTypeKind = iota
    tBool
    tU8
    tU16
    tU32
    tU64
    tU128
    tI8
    tI16
    tI32
    tI64
    tI128
    tF32
    tF64
    tString
    tTimestamp
    tStruct
    tEnum
    tArray
    tMap
)

type rType struct {
    k     rTypeKind
    key   *rType // map key
    elem  *rType // array elem or map value
}

func (t *rType) String() string {
    if t == nil { return "<nil>" }
    switch t.k {
    case tArray:
        return fmt.Sprintf("array<%s>", t.elem)
    case tMap:
        return fmt.Sprintf("map<%s,%s>", t.key, t.elem)
    default:
        return map[rTypeKind]string{
            tNull: "null", tBool: "bool", tU8: "u8", tU16: "u16", tU32: "u32", tU64: "u64", tU128: "u128",
            tI8: "i8", tI16: "i16", tI32: "i32", tI64: "i64", tI128: "i128", tF32: "f32", tF64: "f64",
            tString: "string", tTimestamp: "timestamp", tStruct: "struct", tEnum: "enum",
        }[t.k]
    }
}

func (t *rType) typeID() (byte, bool) {
    switch t.k {
    case tNull:
        return 0x00, true
    case tBool:
        return 0x01, true
    case tU8:
        return 0x02, true
    case tU16:
        return 0x03, true
    case tU32:
        return 0x04, true
    case tU64:
        return 0x05, true
    case tU128:
        return 0x06, true
    case tI8:
        return 0x07, true
    case tI16:
        return 0x08, true
    case tI32:
        return 0x09, true
    case tI64:
        return 0x0A, true
    case tI128:
        return 0x0B, true
    case tF32:
        return 0x0C, true
    case tF64:
        return 0x0D, true
    case tString:
        return 0x0E, true
    case tArray:
        return 0x0F, true
    case tMap:
        return 0x10, true
    case tStruct:
        return 0x11, true
    case tEnum:
        return 0x12, true
    case tTimestamp:
        return 0x13, true
    default:
        return 0, false
    }
}

// values
type value interface{}

type (
    valNull struct{}
    valBool struct{ v bool }
    valStr  struct{ s string }
    valTS   struct{ sec uint64 }
    valInt  struct{ u uint128; i int128; signed bool; bits int }
    valFlt  struct{ v float64; f32 bool }
    valEnum struct{ variant int; inner value }
    valArr  struct{ elem *rType; values []value }
    valMap  struct{ key, val *rType; pairs []kv }
    kv      struct{ k value; v value }
    valStruct struct{ fields []field }
    field struct{ id int; val value; omit bool }
)

// Minimal 128-bit containers
type uint128 struct{ hi, lo uint64 }
type int128 struct{ hi, lo uint64 }

type parser struct { lx *lexer }

func (p *parser) expect(k tokKind) error {
    if p.lx.cur.kind != k {
        return fmt.Errorf("expected %v, got %v (%s)", k, p.lx.cur.kind, p.lx.cur.lit)
    }
    p.lx.next()
    return nil
}

func (p *parser) parseAlias() (alias, error) {
    // current is 'let'
    p.lx.next()
    if p.lx.cur.kind != tokIdent { return alias{}, fmt.Errorf("expected identifier after let") }
    name := p.lx.cur.lit
    p.lx.next()
    if err := p.expect(tokEq); err != nil { return alias{}, err }
    if p.lx.cur.kind != tokInt { return alias{}, fmt.Errorf("expected numeric field id after =") }
    id, err := parseDecInt(p.lx.cur.lit)
    if err != nil { return alias{}, fmt.Errorf("invalid field id: %v", err) }
    p.lx.next()
    var typ *rType
    if p.lx.cur.kind == tokColon {
        p.lx.next()
        t, err := p.parseType()
        if err != nil { return alias{}, err }
        typ = t
    }
    if p.lx.cur.kind == tokSemi { p.lx.next() }
    return alias{name: name, id: id, typ: typ}, nil
}

func (p *parser) parseType() (*rType, error) {
    switch p.lx.cur.kind {
    case tokIdent:
        switch p.lx.cur.lit {
        case "null": p.lx.next(); return &rType{k:tNull}, nil
        case "bool": p.lx.next(); return &rType{k:tBool}, nil
        case "u8": p.lx.next(); return &rType{k:tU8}, nil
        case "u16": p.lx.next(); return &rType{k:tU16}, nil
        case "u32": p.lx.next(); return &rType{k:tU32}, nil
        case "u64": p.lx.next(); return &rType{k:tU64}, nil
        case "u128": p.lx.next(); return &rType{k:tU128}, nil
        case "i8": p.lx.next(); return &rType{k:tI8}, nil
        case "i16": p.lx.next(); return &rType{k:tI16}, nil
        case "i32": p.lx.next(); return &rType{k:tI32}, nil
        case "i64": p.lx.next(); return &rType{k:tI64}, nil
        case "i128": p.lx.next(); return &rType{k:tI128}, nil
        case "f32": p.lx.next(); return &rType{k:tF32}, nil
        case "f64": p.lx.next(); return &rType{k:tF64}, nil
        case "string": p.lx.next(); return &rType{k:tString}, nil
        case "timestamp": p.lx.next(); return &rType{k:tTimestamp}, nil
        case "struct": p.lx.next(); return &rType{k:tStruct}, nil
        case "enum": p.lx.next(); return &rType{k:tEnum}, nil
        case "array":
            p.lx.next()
            if err := p.expect(tokLt); err != nil { return nil, err }
            elem, err := p.parseType()
            if err != nil { return nil, err }
            if err := p.expect(tokGt); err != nil { return nil, err }
            return &rType{k:tArray, elem: elem}, nil
        case "map":
            p.lx.next()
            if err := p.expect(tokLt); err != nil { return nil, err }
            k, err := p.parseType()
            if err != nil { return nil, err }
            if err := p.expect(tokComma); err != nil { return nil, err }
            v, err := p.parseType()
            if err != nil { return nil, err }
            if err := p.expect(tokGt); err != nil { return nil, err }
            return &rType{k:tMap, key: k, elem: v}, nil
        }
    }
    return nil, fmt.Errorf("invalid type")
}

func (p *parser) parseStructLiteral(aliases map[string]alias) (*valStruct, error) {
    if p.lx.cur.kind != tokStruct { return nil, fmt.Errorf("expected struct") }
    p.lx.next()
    if err := p.expect(tokLBrace); err != nil { return nil, err }
    st := &valStruct{}
    for p.lx.cur.kind != tokRBrace && p.lx.cur.kind != tokEOF {
        // FieldKey
        var id int
        if p.lx.cur.kind == tokInt {
            n, err := parseDecInt(p.lx.cur.lit)
            if err != nil { return nil, err }
            id = n
            p.lx.next()
        } else if p.lx.cur.kind == tokIdent {
            name := p.lx.cur.lit
            a, ok := aliases[name]
            if !ok { return nil, fmt.Errorf("unknown field alias: %s", name) }
            id = a.id
            p.lx.next()
        } else {
            return nil, fmt.Errorf("expected field key, got %v", p.lx.cur.kind)
        }
        if err := p.expect(tokColon); err != nil { return nil, err }
        if p.lx.cur.kind == tokNone {
            // explicit omission
            st.fields = append(st.fields, field{id: id, omit: true})
            p.lx.next()
        } else {
            v, err := p.parseValue(nil, aliases)
            if err != nil { return nil, err }
            st.fields = append(st.fields, field{id: id, val: v})
        }
        if p.lx.cur.kind == tokSemi || p.lx.cur.kind == tokComma { p.lx.next() }
    }
    if err := p.expect(tokRBrace); err != nil { return nil, err }
    return st, nil
}

func (p *parser) parseValue(hint *rType, aliases map[string]alias) (value, error) {
    // Cast: (Type) Value
    if p.lx.cur.kind == tokLParen {
        p.lx.next()
        t, err := p.parseType()
        if err != nil { return nil, err }
        if err := p.expect(tokRParen); err != nil { return nil, err }
        return p.parseValue(t, aliases)
    }
    switch p.lx.cur.kind {
    case tokNull:
        p.lx.next(); return valNull{}, nil
    case tokTrue:
        p.lx.next(); return valBool{v: true}, nil
    case tokFalse:
        p.lx.next(); return valBool{v: false}, nil
    case tokString:
        s := p.lx.cur.lit; p.lx.next(); return valStr{s: s}, nil
    case tokTS:
        // ts( NUMBER | STRING )
        p.lx.next()
        if err := p.expect(tokLParen); err != nil { return nil, err }
        var sec uint64
        switch p.lx.cur.kind {
        case tokInt:
            n, err := parseDecInt(p.lx.cur.lit)
            if err != nil || n < 0 { return nil, fmt.Errorf("invalid timestamp int") }
            sec = uint64(n)
            p.lx.next()
        case tokString:
            t, err := time.Parse(time.RFC3339, p.lx.cur.lit)
            if err != nil { return nil, fmt.Errorf("invalid RFC3339 timestamp") }
            sec = uint64(t.Unix())
            p.lx.next()
        default:
            return nil, fmt.Errorf("expected int or string in ts(...)")
        }
        if err := p.expect(tokRParen); err != nil { return nil, err }
        return valTS{sec: sec}, nil
    case tokFloat:
        // default f64, allow suffix f32/f64 by following ident tokens (simplified: accept trailing ident)
        lit := stripUnderscores(p.lx.cur.lit)
        p.lx.next()
        f, err := strconv.ParseFloat(lit, 64)
        if err != nil { return nil, err }
        // Optional type suffix as next token (ident)
        f32 := false
        if p.lx.cur.kind == tokIdent && (p.lx.cur.lit == "f32" || p.lx.cur.lit == "f64") {
            f32 = p.lx.cur.lit == "f32"
            p.lx.next()
        }
        return valFlt{v: f, f32: f32}, nil
    case tokInt:
        // Require explicit integer type via suffix (ident) or hint/cast
        txt := p.lx.cur.lit
        base := p.lx.cur.intBase
        p.lx.next()
        // Optional suffix as next token (ident like u32/i64/u8/u128/...)
        var suf string
        if p.lx.cur.kind == tokIdent {
            suf = p.lx.cur.lit
            switch suf {
            case "u8","u16","u32","u64","u128","i8","i16","i32","i64","i128":
                p.lx.next()
            default:
                suf = ""
            }
        }
        if suf == "" && hint == nil {
            return nil, fmt.Errorf("ambiguous integer literal; add a type suffix or cast")
        }
        if suf == "" && hint != nil {
            suf = hint.String()
        }
        // parse number (allow underscores)
        if base == 10 {
            txt = stripUnderscores(txt)
        } else {
            // keep 0x prefix; remove underscores
            txt = strings.ReplaceAll(txt, "_", "")
        }
        // use big values only as far as 64-bit; 128-bit via parsing into hi/lo
        unsigned := strings.HasPrefix(suf, "u")
        switch suf {
        case "u8", "u16", "u32", "u64":
            u, err := strconv.ParseUint(strings.TrimPrefix(txt, "+"), base, 64)
            if err != nil { return nil, err }
            bits := map[string]int{"u8":8,"u16":16,"u32":32,"u64":64}[suf]
            return valInt{u:uint128{lo:u}, signed:false, bits:bits}, nil
        case "i8", "i16", "i32", "i64":
            i, err := strconv.ParseInt(txt, base, 64)
            if err != nil { return nil, err }
            bits := map[string]int{"i8":8,"i16":16,"i32":32,"i64":64}[suf]
            return valInt{i:int128{lo:uint64(i)}, signed:true, bits:bits}, nil
        case "u128", "i128":
            // crude 128-bit parse via big.Int path avoided; limited support: decimal only
            if base != 10 { return nil, fmt.Errorf("hex 128-bit not supported") }
            txt = strings.TrimPrefix(txt, "+")
            neg := strings.HasPrefix(txt, "-")
            if neg && unsigned { return nil, fmt.Errorf("negative unsigned literal") }
            if neg { txt = strings.TrimPrefix(txt, "-") }
            // split into hi/lo using decimal parsing
            // Simple approach: use math via big ints would be better; to keep footprint, restrict to <= 2^64-1 for u128 and i128 magnitude <= 2^127-1
            // So we parse as uint64 and place in lo; hi must be 0.
            u, err := strconv.ParseUint(txt, 10, 64)
            if err != nil { return nil, fmt.Errorf("128-bit value too large or invalid: %v", err) }
            if unsigned {
                return valInt{u:uint128{hi:0, lo:u}, signed:false, bits:128}, nil
            }
            // signed 128: store two's complement notion in hi/lo; for small values just put in lo.
            if neg {
                // represent negative small as two's complement: (2^64 - |v|) in lo and hi all ones.
                v, _ := strconv.ParseUint(strings.TrimPrefix(txt, "-"), 10, 64)
                lo := (^uint64(0)) - (v - 1)
                return valInt{i:int128{hi:^uint64(0), lo:lo}, signed:true, bits:128}, nil
            }
            return valInt{i:int128{hi:0, lo:u}, signed:true, bits:128}, nil
        default:
            return nil, fmt.Errorf("unsupported integer type suffix: %s", suf)
        }
    case tokArray:
        // array<type>? [ ... ]
        p.lx.next()
        var elem *rType
        if p.lx.cur.kind == tokLt {
            p.lx.next()
            t, err := p.parseType()
            if err != nil { return nil, err }
            elem = t
            if err := p.expect(tokGt); err != nil { return nil, err }
        }
        if err := p.expect(tokLBrack); err != nil { return nil, err }
        var vals []value
        for p.lx.cur.kind != tokRBrack && p.lx.cur.kind != tokEOF {
            v, err := p.parseValue(elem, aliases)
            if err != nil { return nil, err }
            vals = append(vals, v)
            if p.lx.cur.kind == tokComma { p.lx.next() }
        }
        if err := p.expect(tokRBrack); err != nil { return nil, err }
        if elem == nil {
            // infer from first element if any
            if len(vals) == 0 { return valArr{elem: &rType{k:tU8}, values: nil}, nil }
            // best-effort inference based on literal kinds
            switch vals[0].(type) {
            case valStr:
                elem = &rType{k:tString}
            case valBool:
                elem = &rType{k:tBool}
            case valTS:
                elem = &rType{k:tTimestamp}
            case valFlt:
                if vals[0].(valFlt).f32 { elem = &rType{k:tF32} } else { elem = &rType{k:tF64} }
            case valInt:
                vi := vals[0].(valInt)
                if vi.signed {
                    switch vi.bits { case 8: elem = &rType{k:tI8}; case 16: elem = &rType{k:tI16}; case 32: elem = &rType{k:tI32}; case 64: elem = &rType{k:tI64}; default: elem = &rType{k:tI128} }
                } else {
                    switch vi.bits { case 8: elem = &rType{k:tU8}; case 16: elem = &rType{k:tU16}; case 32: elem = &rType{k:tU32}; case 64: elem = &rType{k:tU64}; default: elem = &rType{k:tU128} }
                }
            case valArr, valMap, valEnum, valStruct:
                return nil, fmt.Errorf("cannot infer array element type from complex literal; use array<...>")
            default:
                return nil, fmt.Errorf("cannot infer array element type; add array<type>")
            }
        }
        return valArr{elem: elem, values: vals}, nil
    case tokMap:
        // map<k,v>{ ... }
        p.lx.next()
        if err := p.expect(tokLt); err != nil { return nil, err }
        kt, err := p.parseType()
        if err != nil { return nil, err }
        if err := p.expect(tokComma); err != nil { return nil, err }
        vt, err := p.parseType()
        if err != nil { return nil, err }
        if err := p.expect(tokGt); err != nil { return nil, err }
        if err := p.expect(tokLBrace); err != nil { return nil, err }
        var pairs []kv
        for p.lx.cur.kind != tokRBrace && p.lx.cur.kind != tokEOF {
            k, err := p.parseValue(kt, aliases)
            if err != nil { return nil, err }
            if err := p.expect(tokColon); err != nil { return nil, err }
            v, err := p.parseValue(vt, aliases)
            if err != nil { return nil, err }
            pairs = append(pairs, kv{k:k, v:v})
            if p.lx.cur.kind == tokComma { p.lx.next() }
        }
        if err := p.expect(tokRBrace); err != nil { return nil, err }
        return valMap{key: kt, val: vt, pairs: pairs}, nil
    case tokEnum:
        // enum<id>(value)
        p.lx.next()
        if err := p.expect(tokLt); err != nil { return nil, err }
        if p.lx.cur.kind != tokInt { return nil, fmt.Errorf("expected variant id") }
        vid, err := parseDecInt(p.lx.cur.lit)
        if err != nil || vid < 0 || vid >= 0x80 { return nil, fmt.Errorf("invalid variant id") }
        p.lx.next()
        if err := p.expect(tokGt); err != nil { return nil, err }
        if err := p.expect(tokLParen); err != nil { return nil, err }
        inner, err := p.parseValue(nil, aliases)
        if err != nil { return nil, err }
        if err := p.expect(tokRParen); err != nil { return nil, err }
        return valEnum{variant: vid, inner: inner}, nil
    case tokStruct:
        return p.parseStructLiteral(aliases)
    default:
        return nil, fmt.Errorf("unexpected token in value: %v", p.lx.cur.kind)
    }
}

func parseDecInt(lit string) (int, error) {
    lit = stripUnderscores(lit)
    i, err := strconv.ParseInt(lit, 10, 64)
    if err != nil { return 0, err }
    return int(i), nil
}

// Encoding helpers

func encodeValueTLV(v value) ([]byte, error) {
    var buf bytes.Buffer
    switch x := v.(type) {
    case valNull:
        if err := intr.WriteNullTLV(&buf); err != nil { return nil, err }
    case valBool:
        if err := intr.WriteBoolTLV(&buf, x.v); err != nil { return nil, err }
    case valStr:
        if err := intr.WriteStringTLV(&buf, x.s); err != nil { return nil, err }
    case valTS:
        if err := intr.WriteTimestampTLV(&buf, x.sec); err != nil { return nil, err }
    case valFlt:
        if x.f32 {
            if err := intr.WriteF32TLV(&buf, float32(x.v)); err != nil { return nil, err }
        } else {
            if err := intr.WriteF64TLV(&buf, x.v); err != nil { return nil, err }
        }
    case valInt:
        if x.signed {
            switch x.bits {
            case 8:
                if err := intr.WriteI8TLV(&buf, int8(int64(x.i.lo))); err != nil { return nil, err }
            case 16:
                if err := intr.WriteI16TLV(&buf, int16(int64(x.i.lo))); err != nil { return nil, err }
            case 32:
                if err := intr.WriteI32TLV(&buf, int32(int64(x.i.lo))); err != nil { return nil, err }
            case 64:
                if err := intr.WriteI64TLV(&buf, int64(x.i.lo)); err != nil { return nil, err }
            case 128:
                var b [16]byte
                // little-endian store lo then hi
                putU64Le(b[:8], x.i.lo)
                putU64Le(b[8:], x.i.hi)
                if err := intr.WriteI128TLV(&buf, b); err != nil { return nil, err }
            default:
                return nil, errors.New("invalid signed int bits")
            }
        } else {
            switch x.bits {
            case 8:
                if err := intr.WriteU8TLV(&buf, uint8(x.u.lo)); err != nil { return nil, err }
            case 16:
                if err := intr.WriteU16TLV(&buf, uint16(x.u.lo)); err != nil { return nil, err }
            case 32:
                if err := intr.WriteU32TLV(&buf, uint32(x.u.lo)); err != nil { return nil, err }
            case 64:
                if err := intr.WriteU64TLV(&buf, x.u.lo); err != nil { return nil, err }
            case 128:
                var b [16]byte
                putU64Le(b[:8], x.u.lo)
                putU64Le(b[8:], x.u.hi)
                if err := intr.WriteU128TLV(&buf, b); err != nil { return nil, err }
            default:
                return nil, errors.New("invalid unsigned int bits")
            }
        }
    case valEnum:
        if err := intr.WriteEnumTLV(&buf, byte(x.variant), func(w io.Writer) error {
            tlv, err := encodeValueTLV(x.inner)
            if err != nil { return err }
            _, err = w.Write(tlv)
            return err
        }); err != nil { return nil, err }
    case valArr:
        // determine element type id
        etid, ok := x.elem.typeID()
        if !ok { return nil, fmt.Errorf("unknown array element type: %v", x.elem) }
        if err := intr.WriteArrayTLV(&buf, etid, func(w io.Writer) error {
            for _, ev := range x.values {
                tlv, err := encodeValueTLV(ev)
                if err != nil { return err }
                // drop type byte; write content per fixed/var rules
                if len(tlv) < 1 { return fmt.Errorf("invalid element tlv") }
                if _, err := w.Write(tlv[1:]); err != nil { return err }
            }
            return nil
        }); err != nil { return nil, err }
    case valMap:
        ktid, ok := x.key.typeID(); if !ok { return nil, fmt.Errorf("unknown map key type") }
        vtid, ok := x.val.typeID(); if !ok { return nil, fmt.Errorf("unknown map value type") }
        if err := intr.WriteMapTLV(&buf, ktid, vtid, func(w io.Writer) error {
            // enforce duplicate key check at encode time by exact TLV bytes of keys
            seen := make(map[string]struct{})
            for _, p := range x.pairs {
                ktlv, err := encodeValueTLV(p.k); if err != nil { return err }
                vtlv, err := encodeValueTLV(p.v); if err != nil { return err }
                // key payload without type
                if len(ktlv) < 1 { return fmt.Errorf("invalid key tlv") }
                keyPayload := ktlv[1:]
                if _, ok := seen[string(keyPayload)]; ok { return fmt.Errorf("duplicate map key") }
                seen[string(keyPayload)] = struct{}{}
                if _, err := w.Write(keyPayload); err != nil { return err }
                if len(vtlv) < 1 { return fmt.Errorf("invalid value tlv") }
                if _, err := w.Write(vtlv[1:]); err != nil { return err }
            }
            return nil
        }); err != nil { return nil, err }
    case *valStruct:
        // nested struct
        if err := intr.WriteStructTLV(&buf, func(w io.Writer) error {
            sort.Slice(x.fields, func(i, j int) bool { return x.fields[i].id < x.fields[j].id })
            prev := -1
            for _, f := range x.fields {
                if f.omit { continue }
                if f.id <= prev { return fmt.Errorf("struct field ids not strictly increasing in nested struct") }
                prev = f.id
                if err := intr.WriteType(w, byte(f.id)); err != nil { return err }
                tlv, err := encodeValueTLV(f.val)
                if err != nil { return err }
                if _, err := w.Write(tlv); err != nil { return err }
            }
            return nil
        }); err != nil { return nil, err }
    default:
        return nil, fmt.Errorf("unsupported value kind: %T", v)
    }
    return buf.Bytes(), nil
}

func putU64Le(dst []byte, v uint64) {
    dst[0] = byte(v)
    dst[1] = byte(v >> 8)
    dst[2] = byte(v >> 16)
    dst[3] = byte(v >> 24)
    dst[4] = byte(v >> 32)
    dst[5] = byte(v >> 40)
    dst[6] = byte(v >> 48)
    dst[7] = byte(v >> 56)
}

