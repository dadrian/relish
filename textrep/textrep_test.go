package textrep

import (
    "bytes"
    "testing"

    intr "github.com/dadrian/relish/internal"
)

func TestEncode_SimpleStruct(t *testing.T) {
    src := []byte(`
        let id = 1: u64;
        let name = 2: string;
        struct { id: 42u64; name: "Ada"; }
    `)
    out, err := EncodeBytes(src)
    if err != nil { t.Fatalf("EncodeBytes error: %v", err) }
    // Basic sanity: starts with struct type id 0x11
    if len(out) == 0 || out[0] != 0x11 {
        t.Fatalf("expected struct type id 0x11, got %x", out)
    }
    // Read struct payload and check two fields present with increasing IDs
    br := bytes.NewReader(out)
    if _, err := intr.ReadType(br); err != nil { t.Fatal(err) }
    n, _, err := intr.ReadLen(br)
    if err != nil { t.Fatal(err) }
    payload := make([]byte, n)
    if err := intr.ReadFull(br, payload); err != nil { t.Fatal(err) }
    r := bytes.NewReader(payload)
    // field 1
    b, _ := r.ReadByte()
    if int(b) != 1 { t.Fatalf("want field id 1, got %d", int(b)) }
    // u64 TLV
    if v, err := intr.ReadU64TLV(r); err != nil || v != 42 { t.Fatalf("u64 read err=%v v=%d", err, v) }
    // field 2
    b, _ = r.ReadByte()
    if int(b) != 2 { t.Fatalf("want field id 2, got %d", int(b)) }
    s, err := intr.ReadStringTLV(r)
    if err != nil || s != "Ada" { t.Fatalf("string read err=%v s=%q", err, s) }
    if r.Len() != 0 { t.Fatalf("extra bytes remaining: %d", r.Len()) }
}

func TestEncode_ArrayStrings(t *testing.T) {
    src := []byte(`struct { 10: array<string>["a","b","c"]; }`)
    out, err := EncodeBytes(src)
    if err != nil { t.Fatalf("EncodeBytes error: %v", err) }
    br := bytes.NewReader(out)
    if _, err := intr.ReadType(br); err != nil { t.Fatal(err) }
    n, _, err := intr.ReadLen(br)
    if err != nil { t.Fatal(err) }
    payload := make([]byte, n)
    if err := intr.ReadFull(br, payload); err != nil { t.Fatal(err) }
    r := bytes.NewReader(payload)
    // field 10
    b, _ := r.ReadByte()
    if int(b) != 10 { t.Fatalf("want field id 10, got %d", int(b)) }
    // array tlv
    et, arrPayload, err := intr.ReadArrayTLV(r)
    if err != nil { t.Fatal(err) }
    if et != 0x0E { t.Fatalf("want string elem type 0x0E, got 0x%02x", et) }
    // iterate elements: each is [len][content]
    rr := bytes.NewReader(arrPayload)
    for i, want := range []string{"a","b","c"} {
        n, _, err := intr.ReadLen(rr)
        if err != nil { t.Fatal(err) }
        buf := make([]byte, n)
        if err := intr.ReadFull(rr, buf); err != nil { t.Fatal(err) }
        if string(buf) != want { t.Fatalf("elem %d = %q, want %q", i, string(buf), want) }
    }
    if r.Len() != 0 { t.Fatalf("extra bytes remaining: %d", r.Len()) }
}

