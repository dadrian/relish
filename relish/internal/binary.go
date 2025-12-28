package internal

import (
	"encoding/binary"
	"io"
)

var le = binary.LittleEndian

func WriteU16(w io.Writer, v uint16) error {
	var b [2]byte
	le.PutUint16(b[:], v)
	_, err := w.Write(b[:])
	return err
}
func WriteU32(w io.Writer, v uint32) error {
	var b [4]byte
	le.PutUint32(b[:], v)
	_, err := w.Write(b[:])
	return err
}
func WriteU64(w io.Writer, v uint64) error {
	var b [8]byte
	le.PutUint64(b[:], v)
	_, err := w.Write(b[:])
	return err
}

func WriteI16(w io.Writer, v int16) error { return WriteU16(w, uint16(v)) }
func WriteI32(w io.Writer, v int32) error { return WriteU32(w, uint32(v)) }
func WriteI64(w io.Writer, v int64) error { return WriteU64(w, uint64(v)) }

func ReadFull(r io.Reader, buf []byte) error {
	var off int
	for off < len(buf) {
		n, err := r.Read(buf[off:])
		if n > 0 {
			off += n
		}
		if err != nil {
			if off == len(buf) {
				return nil
			}
			return err
		}
		if n == 0 {
			// io.Reader made no progress; avoid infinite loop
			return io.ErrUnexpectedEOF
		}
	}
	return nil
}
