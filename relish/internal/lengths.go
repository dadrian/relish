package internal

// Tagged-varint length encoding per SPEC.md.
// Short form: 1 byte, LSB=0, upper 7 bits carry length (0..127).
// Long form: 4 bytes, first byte LSB=1, remaining 31 bits little-endian
// across first byte's upper 7 bits and next 3 bytes (max 2^31-1).

const MaxLen = 1<<31 - 1

// SizeOfLen returns the number of bytes needed to encode n.
func SizeOfLen(n int) int {
	if n < 0 || n > MaxLen {
		return -1
	}
	if n <= 0x7F { // 0..127
		return 1
	}
	return 4
}

// EncodeLen encodes n into dst and returns the number of bytes written.
// dst must have length >= SizeOfLen(n).
func EncodeLen(dst []byte, n int) int {
	if n <= 0x7F {
		dst[0] = byte((n << 1) & 0xFE) // ensure LSB=0
		return 1
	}
	// Long form: [b0][b1][b2][b3]
	// 31-bit length, little-endian; b0 carries low 7 bits in bits 7..1, bit0=1
	u := uint32(n)
	dst[0] = byte(((u & 0x7F) << 1) | 0x01) // low 7 bits and tag bit
	dst[1] = byte((u >> 7) & 0xFF)
	dst[2] = byte((u >> 15) & 0xFF)
	dst[3] = byte((u >> 23) & 0xFF)
	return 4
}

// DecodeLen decodes a tagged-varint length from src, returning value and bytes consumed.
// It returns (-1, 0) on malformed or out-of-range input.
func DecodeLen(src []byte) (int, int) {
	if len(src) == 0 {
		return -1, 0
	}
	b0 := src[0]
	if b0&0x01 == 0 { // short form
		n := int(b0 >> 1)
		return n, 1
	}
	if len(src) < 4 {
		return -1, 0
	}
	// long form; reconstruct 31-bit little-endian value
	n := int((uint32(b0 >> 1)) | (uint32(src[1]) << 7) | (uint32(src[2]) << 15) | (uint32(src[3]) << 23))
	if n < 0 || n > MaxLen {
		return -1, 0
	}
	return n, 4
}
