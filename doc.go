// Package relish provides encoding and decoding for the Relish
// serialization format described in SPEC.md.
//
// This package exposes high-level Marshal/Unmarshal helpers as well as
// streaming Encoder/Decoder types. The implementation targets correctness
// and explicit compatibility per the spec (field IDs, TLV model, etc.).
package relish
