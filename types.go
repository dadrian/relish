package relish

// TypeID identifies a Relish type. Top bit must be 0 per spec.
type TypeID byte

const (
	TypeNull      TypeID = 0x00
	TypeBool      TypeID = 0x01
	TypeU8        TypeID = 0x02
	TypeU16       TypeID = 0x03
	TypeU32       TypeID = 0x04
	TypeU64       TypeID = 0x05
	TypeU128      TypeID = 0x06
	TypeI8        TypeID = 0x07
	TypeI16       TypeID = 0x08
	TypeI32       TypeID = 0x09
	TypeI64       TypeID = 0x0A
	TypeI128      TypeID = 0x0B
	TypeF32       TypeID = 0x0C
	TypeF64       TypeID = 0x0D
	TypeString    TypeID = 0x0E
	TypeArray     TypeID = 0x0F
	TypeMap       TypeID = 0x10
	TypeStruct    TypeID = 0x11
	TypeEnum      TypeID = 0x12
	TypeTimestamp TypeID = 0x13
)

// Null represents the Relish Null value.
type Null struct{}

// U128 and I128 are 128-bit integer containers represented as bytes.
// The on-wire representation is little-endian.
type U128 [16]byte
type I128 [16]byte
