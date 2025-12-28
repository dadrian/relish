package relish

import "fmt"

// ErrorKind classifies decoding/encoding errors.
type ErrorKind int

const (
	ErrInvalidTypeID ErrorKind = iota + 1
	ErrInvalidFieldID
	ErrFieldOrder
	ErrDuplicateMapKey
	ErrInvalidUTF8
	ErrLengthOverflow
	ErrUnexpectedEOF
	ErrTypeMismatch
	ErrEnumLengthMismatch
	ErrNotImplementedKind
)

// Error carries offset and classification for better diagnostics.
type Error struct {
	Offset int64
	Kind   ErrorKind
	Detail string
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Offset > 0 {
		return fmt.Sprintf("relish: %v at %d: %s", e.Kind, e.Offset, e.Detail)
	}
	return fmt.Sprintf("relish: %v: %s", e.Kind, e.Detail)
}

// ErrNotImplemented is returned by stubbed methods.
var ErrNotImplemented = &Error{Kind: ErrNotImplementedKind, Detail: "not implemented"}
